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
//  3. Direct link (QR code on the animal page): GET /guest with number +
//     token + lang opens the status view immediately — the token is a salted
//     hash of the recorded discoverer phone (guestPhoneToken), so the phone
//     number itself never appears in the URL. lang selects the page language
//     before rendering; the QR code is generated in the current UI language.
//  4. On success a succinct view is rendered:
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
// request-scoped tspecies helper. Guest pages use the minimal guest.plush.html
// layout: the only menu is the language selector.
//
// Abuse protection: a per-instance in-memory rate limiter caps the number of
// verification attempts per client IP (guestRateMax within guestRateWindow).

import (
	cryptosha256 "crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
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
// Phone token (salted hash for the direct QR-code link)
// ---------------------------------------------------------------------------

// guestPhoneToken returns a hex-encoded SHA-256 hash of the phone number,
// salted with the animal number. The token is embedded in the URL encoded in
// the QR code on the animal page, so that scanning the code opens the guest
// status view directly (no form, no phone entry) while the raw phone number
// never appears in the URL. Knowing the animal number alone is not enough to
// forge a token — the discoverer's phone number is required.
func guestPhoneToken(animalNumber, phone string) string {
	sum := cryptosha256.Sum256([]byte("creaves-guest:" +
		guestNormalizePhone(animalNumber) + ":" + guestNormalizePhone(phone)))
	return hex.EncodeToString(sum[:])
}

// guestTokenMatches compares the given token with the expected one in
// constant time. Empty tokens never match.
func guestTokenMatches(expected, given string) bool {
	if expected == "" || given == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(given)) == 1
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

// guestLayout is the minimal layout used for the public guest pages: it shows
// only the language selector — no application menu.
const guestLayout = "guest.plush.html"

// guestRenderNew renders the guest form with the minimal guest layout.
func guestRenderNew(c buffalo.Context, status int) error {
	return c.Render(status, r.HTML("guest/new.plush.html", guestLayout))
}

// guestRenderShow renders the guest status view with the minimal guest layout.
func guestRenderShow(c buffalo.Context) error {
	return c.Render(http.StatusOK, r.HTML("guest/show.plush.html", guestLayout))
}

// guestLangTarget returns the current request URI without the lang parameter,
// so language switches on guest pages do not re-apply the previous language.
func guestLangTarget(c buffalo.Context) string {
	u := *c.Request().URL
	q := u.Query()
	q.Del("lang")
	u.RawQuery = q.Encode()
	return u.RequestURI()
}

// applyGuestLang applies the lang query parameter (from the QR code URL):
// it validates the code, persists it in the lang cookie and refreshes the
// request language so the localized template variants are selected.
func applyGuestLang(c buffalo.Context, lang string) {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		return
	}
	for _, l := range uiLanguages {
		if l.code != lang {
			continue
		}
		cookie := http.Cookie{
			Name:   "lang",
			Value:  lang,
			MaxAge: int((time.Hour * 24 * 265).Seconds()),
			Path:   "/",
		}
		http.SetCookie(c.Response(), &cookie)
		T.Refresh(c, lang)
		return
	}
}

// guestStoredPhone loads the discoverer phone recorded for the animal.
// Returns "" when the discovery has no discoverer or no phone number.
func guestStoredPhone(tx *pop.Connection, a *models.Animal) (string, error) {
	d := models.Discovery{}
	if err := tx.Eager("Discoverer").Find(&d, a.DiscoveryID); err != nil {
		return "", err
	}
	if !d.Discoverer.Phone.Valid {
		return "", nil
	}
	return d.Discoverer.Phone.String, nil
}

// GuestNew renders the public guest lookup form. GET /guest
//
// Direct link (QR code on the animal page): with number, token (salted hash
// of the discoverer phone) and lang the status view is rendered immediately,
// without any further interaction. lang is applied before rendering so the
// scanned code opens the page in the encoded language.
func GuestNew(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	c.Set("guestLangTarget", guestLangTarget(c))
	applyGuestLang(c, c.Param("lang"))

	number := strings.TrimSpace(c.Param("number"))
	number = strings.TrimSpace(strings.ReplaceAll(number, " ", ""))
	if len(number) > 20 {
		number = number[:20]
	}
	c.Set("animalNumber", number)

	// Direct link: animal number + phone token. On any failure the plain
	// form is shown — no information is disclosed about the reason.
	token := strings.TrimSpace(c.Param("token"))
	if number != "" && token != "" {
		// Rate limit direct lookups like form submissions.
		ip := c.Request().RemoteAddr
		if xff := c.Request().Header.Get("X-Forwarded-For"); xff != "" {
			ip = strings.TrimSpace(strings.Split(xff, ",")[0])
		}
		if !guestRateAllow(ip, time.Now()) {
			c.Logger().Warn("guest: rate limit reached for", ip)
			c.Set("guestError", false)
			c.Set("guestRateLimited", true)
			return guestRenderNew(c, http.StatusTooManyRequests)
		}

		a, err := guestFindAnimal(tx, number)
		if err != nil {
			return err
		}
		if a != nil {
			stored, err := guestStoredPhone(tx, a)
			if err != nil {
				return err
			}
			expected := guestPhoneToken(a.YearNumberFormatted(), stored)
			if guestTokenMatches(expected, token) {
				view, err := buildGuestView(tx, a, time.Now())
				if err != nil {
					return err
				}
				c.Set("guest", view)
				return guestRenderShow(c)
			}
		}
	}

	c.Set("guest", nil)
	c.Set("guestError", false)
	c.Set("guestRateLimited", false)
	return guestRenderNew(c, http.StatusOK)
}

// GuestCreate verifies the animal number + phone number combination and
// renders the succinct status view. POST /guest
func GuestCreate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	c.Set("guestLangTarget", guestLangTarget(c))

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
			stored, err := guestStoredPhone(tx, a)
			if err != nil {
				return err
			}
			if stored != "" && guestPhoneMatches(stored, phone) {
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
		return guestRenderShow(c)
	}

	c.Set("guest", nil)
	c.Set("guestError", !rateLimited)
	c.Set("guestRateLimited", rateLimited)
	c.Set("animalNumber", number)
	status := http.StatusOK
	if rateLimited {
		status = http.StatusTooManyRequests
	}
	return guestRenderNew(c, status)
}

// ---------------------------------------------------------------------------
// QR code (link from the animal page to the guest status view)
// ---------------------------------------------------------------------------

// guestStatusURL builds the public URL of the guest status view. With a
// non-empty token the URL opens the status view directly (QR code on the
// animal page); with an empty token it falls back to the plain form link.
// The language is encoded in the URL so the scanned page opens in the
// language the QR code was generated in.
func guestStatusURL(scheme, host, number, token, lang string) string {
	u := fmt.Sprintf("%s://%s/guest/?number=%s", scheme, host, url.QueryEscape(number))
	if token != "" {
		u += "&token=" + url.QueryEscape(token)
	}
	if lang != "" {
		u += "&lang=" + url.QueryEscape(lang)
	}
	return u
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

// guestRequestLang returns the current UI language of the request (lang
// cookie), defaulting to French — the canonical base language.
func guestRequestLang(c buffalo.Context) string {
	cookie, err := c.Request().Cookie("lang")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return "fr"
}

// AnimalQR renders a QR code (PNG) that opens the guest status view for this
// animal directly: the URL carries a salted hash (token) of the discoverer
// phone instead of the phone number itself, plus the current UI language.
// Without a recorded phone number the QR code falls back to the plain form
// link. GET /animals/{animal_id}/qr.png
func AnimalQR(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return errors.New("no database connection in context")
	}
	animal := &models.Animal{}
	if err := tx.Find(animal, c.Param("animal_id")); err != nil {
		return err
	}
	number := animal.YearNumberFormatted()

	phone, err := guestStoredPhone(tx, animal)
	if err != nil {
		return err
	}
	token := ""
	if phone != "" {
		token = guestPhoneToken(number, phone)
	}

	u := guestStatusURL(guestScheme(c.Request()), c.Request().Host, number, token, guestRequestLang(c))
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
