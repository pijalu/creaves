package actions

// Guest status view — a public (unauthenticated) landing page where the
// discoverer of an animal can look up its status.
//
// Flow:
//  1. GET /guest renders a form asking for the animal number ("123" or
//     "123/24") and the phone number given when the animal was brought in.
//  2. POST /guest looks up the animal by year number, then verifies that the
//     given phone number matches the discoverer phone recorded for the animal
//     (see guestPhoneMatches). On mismatch (or unknown animal) the form is
//     re-rendered with a generic error — no information is disclosed about
//     whether the animal number exists.
//  3. On success a succinct view is rendered:
//     - arrival (intake) date and species;
//     - if the animal is still in the center: a care-intensity status
//       ("In critical care" / "In intensive care" / "In care") derived from
//       the planned feeding/treatment for today;
//     - if the animal has left: the "News for Discoverer" text configured on
//       the outtake type of the recorded outtake.
//
// Multilingual: template variants guest/new.plush.{,fr,de,nl}.html and
// guest/show.plush.{,fr,de,nl}.html are selected by the request language
// (same mechanism as the landing page). Species names are localized with the
// request-scoped tspecies helper.
//
// Abuse protection: a per-instance in-memory rate limiter caps the number of
// verification attempts per client IP (guestRateMax within guestRateWindow).

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/skip2/go-qrcode"
)

// ---------------------------------------------------------------------------
// Rate limiting (per-instance, in-memory)
// ---------------------------------------------------------------------------

const (
	guestRateMax    = 30               // max attempts ...
	guestRateWindow = 15 * time.Minute // ... per window per client IP
)

var (
	guestRateMu   sync.Mutex
	guestRateHits = map[string][]time.Time{}
)

// guestRateAllow records an attempt and reports whether the client may proceed.
func guestRateAllow(key string, now time.Time) bool {
	guestRateMu.Lock()
	defer guestRateMu.Unlock()

	hits := guestRateHits[key]
	fresh := hits[:0]
	for _, ts := range hits {
		if now.Sub(ts) < guestRateWindow {
			fresh = append(fresh, ts)
		}
	}
	if len(fresh) >= guestRateMax {
		guestRateHits[key] = fresh
		return false
	}
	guestRateHits[key] = append(fresh, now)
	return true
}

// ---------------------------------------------------------------------------
// Phone verification
// ---------------------------------------------------------------------------

// guestNormalizePhone reduces a phone number to its digits only.
func guestNormalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// guestPhoneMinSuffix is the minimum number of digits for suffix matching, so
// callers can omit or add country codes while short fragments stay rejected.
const guestPhoneMinSuffix = 6

// guestPhoneMatches reports whether the given phone number matches the stored
// discoverer phone. Digits-only comparison, with a suffix match (>= 6 digits)
// in both directions to tolerate added/stripped country codes.
func guestPhoneMatches(stored, given string) bool {
	ns := guestNormalizePhone(stored)
	ng := guestNormalizePhone(given)
	if ns == "" || ng == "" {
		return false
	}
	if ns == ng {
		return true
	}
	if len(ng) >= guestPhoneMinSuffix && strings.HasSuffix(ns, ng) {
		return true
	}
	if len(ns) >= guestPhoneMinSuffix && strings.HasSuffix(ng, ns) {
		return true
	}
	// national vs international notation: 0612345678 <-> +33612345678
	if len(ns) >= 9 && len(ng) >= 9 && lastDigits(ns, 9) == lastDigits(ng, 9) {
		return true
	}
	return false
}

// lastDigits returns the last n characters of s.
func lastDigits(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// ---------------------------------------------------------------------------
// Animal lookup
// ---------------------------------------------------------------------------

// guestParseAnimalNumber parses "123" or "123/24" (year number with optional
// 2-digit year). Returns yearNumber, year (0 = any year), ok.
func guestParseAnimalNumber(s string) (int, int, bool) {
	m := AnimalYearNumberRegEx.FindStringSubmatch(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
	if m == nil {
		return 0, 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, false
	}
	year := 0
	if len(m) == 4 && len(m[3]) == 2 {
		year, _ = strconv.Atoi("20" + m[3])
	}
	return n, year, true
}

// guestFindAnimal looks up an animal by its year number ("123" or "123/24").
// Returns (nil, nil) when the number is invalid or unknown. When the year is
// omitted the most recent matching animal wins (same behaviour as the
// AnimalsResource list search).
func guestFindAnimal(tx *pop.Connection, number string) (*models.Animal, error) {
	n, year, ok := guestParseAnimalNumber(number)
	if !ok {
		return nil, nil
	}
	q := tx.Where("yearNumber = ?", n)
	if year > 0 {
		q = q.Where("year = ?", year)
	}
	a := models.Animal{}
	if err := q.Order("id desc").First(&a); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// ---------------------------------------------------------------------------
// Care-intensity status ("In care" / "In intensive care" / "In critical care")
// ---------------------------------------------------------------------------

type guestCareStatus int

const (
	guestCareInCare guestCareStatus = iota
	guestCareIntensive
	guestCareCritical
)

func (s guestCareStatus) String() string {
	switch s {
	case guestCareCritical:
		return "critical"
	case guestCareIntensive:
		return "intensive"
	default:
		return "care"
	}
}

type guestCareInputs struct {
	ForceFeed       bool // force-fed animal
	FeedingLate     bool // next planned feeding is overdue (> half a period)
	HasTreatToday   bool // at least one treatment planned today
	MissedSlotToday bool // a planned treatment slot today passed without being done
}

// decideGuestCareStatus maps the care inputs to the guest status.
// Critical dominates, then intensive.
func decideGuestCareStatus(in guestCareInputs) guestCareStatus {
	if in.ForceFeed || in.FeedingLate || in.MissedSlotToday {
		return guestCareCritical
	}
	if in.HasTreatToday {
		return guestCareIntensive
	}
	return guestCareInCare
}

// guestSlotMissed reports whether a treatment slot planned for today was not
// marked done although its cutoff time passed. Slot cutoffs (local time):
// morning 12:00, noon 17:00, evening 23:59 (end of day).
func guestSlotMissed(now time.Time, timebitmap, timedonebitmap int) bool {
	slots := []struct {
		bit       int
		hour, min int
	}{
		{models.Treatement_MORNING, 12, 0},
		{models.Treatement_NOON, 17, 0},
		{models.Treatement_EVENING, 23, 59},
	}
	for _, s := range slots {
		if timebitmap&s.bit == 0 || timedonebitmap&s.bit != 0 {
			continue
		}
		cutoff := time.Date(now.Year(), now.Month(), now.Day(), s.hour, s.min, 0, 0, now.Location())
		if !now.Before(cutoff) {
			return true
		}
	}
	return false
}

// guestCareInfo computes the guest care status of an animal still in care,
// plus the (optional) next planned feeding time formatted "15:04".
func guestCareInfo(tx *pop.Connection, a *models.Animal, now time.Time) (guestCareStatus, string, error) {
	// Treatments planned for today
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.Add(24 * time.Hour)
	trs := models.Treatments{}
	if err := tx.Where("animal_id = ? and date >= ? and date < ?", a.ID, dayStart, dayEnd).All(&trs); err != nil {
		return guestCareInCare, "", err
	}
	in := guestCareInputs{ForceFeed: a.ForceFeed}
	for _, tr := range trs {
		if tr.Timebitmap == 0 {
			continue
		}
		in.HasTreatToday = true
		if guestSlotMissed(now, tr.Timebitmap, tr.Timedonebitmap) {
			in.MissedSlotToday = true
		}
	}

	// Feeding: reuse the landing-page feeding calculation. Only animals with
	// a complete feeding configuration are considered.
	afRaw := []AnimalFeeding{}
	err := tx.RawQuery(`SELECT a.id, a.feeding_start, a.feeding_end, a.feeding_period, a.force_feed, MAX(c.date) AS last_feeding
FROM animals a
LEFT JOIN cares c ON (a.id = c.animal_id AND c.type_id IN (SELECT id FROM caretypes WHERE type=1))
WHERE a.id = ? AND a.feeding_start IS NOT NULL AND a.feeding_end IS NOT NULL AND a.feeding_period > 0
GROUP BY a.id`, a.ID).All(&afRaw)
	if err != nil {
		return guestCareInCare, "", err
	}
	nextFeeding := ""
	if len(afRaw) == 1 {
		calc := calculateFeeding(afRaw[0], now)
		if calc.NextFeeding.Valid {
			// code 0: the planned feeding is overdue by more than half a period
			if calc.NextFeedingCode == 0 {
				in.FeedingLate = true
			}
			nextFeeding = calc.NextFeeding.Time.Format("15:04")
		}
	}

	return decideGuestCareStatus(in), nextFeeding, nil
}

// ---------------------------------------------------------------------------
// Guest view
// ---------------------------------------------------------------------------

type guestView struct {
	AnimalNumber string // "17/24"
	Species      string // canonical species name (localized via tspecies in template)
	ArrivalDate  string // intake date (fallback: discovery date), formatted

	Present bool // animal still in the center

	// Set when Present
	CareStatus  string // "care" | "intensive" | "critical"
	NextFeeding string // "15:04" or ""

	// Set when !Present
	OuttakeDate string
	OuttakeNews string // "News for Discoverer" text of the outtake type
}

func buildGuestView(tx *pop.Connection, a *models.Animal, now time.Time) (*guestView, error) {
	v := &guestView{
		AnimalNumber: fmt.Sprintf("%d/%02d", a.YearNumber, a.Year%100),
		Species:      a.Species,
		Present:      !a.OuttakeID.Valid,
	}

	// Arrival: intake date, falling back to the discovery date.
	var arrival time.Time
	if a.IntakeID != uuid.Nil {
		in := models.Intake{}
		if err := tx.Find(&in, a.IntakeID); err == nil {
			arrival = in.Date
		}
	}
	if arrival.IsZero() {
		d := models.Discovery{}
		if err := tx.Find(&d, a.DiscoveryID); err == nil {
			arrival = d.Date
		}
	}
	v.ArrivalDate = arrival.Format(models.DateFormat)

	if v.Present {
		st, next, err := guestCareInfo(tx, a, now)
		if err != nil {
			return nil, err
		}
		v.CareStatus = st.String()
		v.NextFeeding = next
		return v, nil
	}

	// Animal left: show "News for Discoverer" of the outtake type.
	o := models.Outtake{}
	if err := tx.Eager("Type").Find(&o, a.OuttakeID.UUID); err != nil {
		return nil, err
	}
	v.OuttakeDate = o.Date.Format(models.DateFormat)
	if o.Type.DiscovererNews.Valid && o.Type.DiscovererNews.String != "" {
		v.OuttakeNews = o.Type.DiscovererNews.String
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// GuestNew renders the public guest lookup form. GET /guest
func GuestNew(c buffalo.Context) error {
	c.Set("guestError", false)
	c.Set("guestRateLimited", false)
	// Optional ?number= prefill (used by the QR code on the animal page).
	// Rendered through <%= %> so it is HTML-escaped by plush.
	number := c.Request().URL.Query().Get("number")
	number = strings.TrimSpace(number)
	if len(number) > 20 {
		number = number[:20]
	}
	c.Set("animalNumber", number)
	return c.Render(http.StatusOK, r.HTML("guest/new.plush.html"))
}

// GuestCreate verifies the animal number + phone number combination and
// renders the succinct status view. POST /guest
func GuestCreate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Rate limit per client IP (behind a proxy the first X-Forwarded-For entry wins)
	ip := c.Request().RemoteAddr
	if xff := c.Request().Header.Get("X-Forwarded-For"); xff != "" {
		ip = strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	rateLimited := false
	if !guestRateAllow(ip, time.Now()) {
		c.Logger().Warn("guest: rate limit reached for", ip)
		rateLimited = true
	}

	number := strings.TrimSpace(c.Param("animal_number"))
	phone := c.Param("phone")

	var view *guestView
	if !rateLimited && number != "" && phone != "" {
		a, err := guestFindAnimal(tx, number)
		if err != nil {
			return err
		}
		if a != nil {
			d := models.Discovery{}
			if err := tx.Eager("Discoverer").Find(&d, a.DiscoveryID); err != nil {
				return err
			}
			if d.Discoverer.Phone.Valid && guestPhoneMatches(d.Discoverer.Phone.String, phone) {
				view, err = buildGuestView(tx, a, time.Now())
				if err != nil {
					return err
				}
			}
		}
	}

	if view != nil {
		c.Set("guest", view)
		c.Set("guestError", false)
		c.Set("guestRateLimited", false)
		return c.Render(http.StatusOK, r.HTML("guest/show.plush.html"))
	}

	c.Set("guest", nil)
	c.Set("guestError", !rateLimited)
	c.Set("guestRateLimited", rateLimited)
	c.Set("animalNumber", number)
	status := http.StatusOK
	if rateLimited {
		status = http.StatusTooManyRequests
	}
	return c.Render(status, r.HTML("guest/new.plush.html"))
}

// ---------------------------------------------------------------------------
// QR code (link from the animal page to the guest status form)
// ---------------------------------------------------------------------------

// guestStatusURL builds the public URL of the guest form pre-filled with the
// given animal number. The QR code shown on the animal page points here.
func guestStatusURL(scheme, host, number string) string {
	return fmt.Sprintf("%s://%s/guest/?number=%s", scheme, host, url.QueryEscape(number))
}

// guestScheme guesses the public scheme of the request, honouring the
// X-Forwarded-Proto header set by reverse proxies.
func guestScheme(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}
	if req.Header.Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

// AnimalQR renders a QR code (PNG) that points to the guest status form
// pre-filled with this animal's number. GET /animals/{animal_id}/qr.png
func AnimalQR(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return errors.New("no database connection in context")
	}
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return err
	}
	u := guestStatusURL(guestScheme(c.Request()), c.Request().Host, animal.YearNumberFormatted())
	qr, err := qrcode.New(u, qrcode.Medium)
	if err != nil {
		return err
	}
	png, err := qr.PNG(256)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type", "image/png")
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write(png)
	return err
}
