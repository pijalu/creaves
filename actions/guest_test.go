package actions

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// guest.go: phone normalization + matching (pure functions)
// ---------------------------------------------------------------------------

// TestGuestQRRequestURL pins the QR/general URL composition: scheme and host
// must come from the incoming request (incl. X-Forwarded-Proto), so the QR
// code always points at the same host the staff page was loaded from.
func TestGuestQRRequestURL(t *testing.T) {
	r := httptest.NewRequest("GET", "/animals/33/qr.png", nil)
	r.Host = "creaves.example.org"
	r.Header.Set("X-Forwarded-Proto", "https")

	got := guestStatusURL(guestScheme(r), r.Host, "33/21", "abc123", "fr")
	want := "https://creaves.example.org/guest/?number=33%2F21&token=abc123&lang=fr"
	if got != want {
		t.Fatalf("guestStatusURL = %q, want %q", got, want)
	}

	// Same request without forwarded proto and behind TLS-less dev server.
	r2 := httptest.NewRequest("GET", "/animals/33/qr.png", nil)
	r2.Host = "192.168.1.10:3000"
	got2 := guestStatusURL(guestScheme(r2), r2.Host, "33/21", "", "")
	want2 := "http://192.168.1.10:3000/guest/?number=33%2F21"
	if got2 != want2 {
		t.Fatalf("guestStatusURL = %q, want %q", got2, want2)
	}
}

func TestGuestNormalizePhone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"+33 6 12 34 56 78", "33612345678"},
		{"06.12-34/56", "06123456"},
		{"(0)6 12 34", "061234"},
		{"", ""},
		{"abc", ""},
	}
	for _, c := range cases {
		if got := guestNormalizePhone(c.in); got != c.want {
			t.Errorf("guestNormalizePhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGuestPhoneMatches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stored string
		given  string
		want   bool
	}{
		{"06 12 34 56 78", "0612345678", true},        // exact after normalization
		{"+33 6 12 34 56 78", "06 12 34 56 78", true}, // national vs international (last 9 digits)
		{"06 12 34 56 78", "33612345678", true},       // national vs international (last 9 digits)
		{"+33 6 12 34 56 78", "33612345678", true},    // exact
		{"06 12 34 56 78", "78", false},               // suffix too short
		{"06 12 34 56 78", "06 12 34 99 99", false},   // different number
		{"", "0612345678", false},                     // nothing stored
		{"0612345678", "", false},                     // nothing given
		{"abc", "def", false},                         // no digits at all
	}
	for _, c := range cases {
		if got := guestPhoneMatches(c.stored, c.given); got != c.want {
			t.Errorf("guestPhoneMatches(%q, %q) = %v, want %v", c.stored, c.given, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// guest.go: care status decision (pure functions)
// ---------------------------------------------------------------------------

func TestDecideGuestCareStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   guestCareInputs
		want guestCareStatus
	}{
		{guestCareInputs{}, guestCareInCare},
		{guestCareInputs{HasTreatToday: true}, guestCareIntensive},
		{guestCareInputs{ForceFeed: true}, guestCareCritical},
		{guestCareInputs{FeedingLate: true}, guestCareCritical},
		{guestCareInputs{MissedSlotToday: true}, guestCareCritical},
		// critical dominates intensive
		{guestCareInputs{HasTreatToday: true, FeedingLate: true}, guestCareCritical},
	}
	for _, c := range cases {
		if got := decideGuestCareStatus(c.in); got != c.want {
			t.Errorf("decideGuestCareStatus(%+v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestGuestSlotMissed(t *testing.T) {
	t.Parallel()
	loc := time.Local
	mk := func(h, m int) time.Time { return time.Date(2024, 6, 1, h, m, 0, 0, loc) }

	cases := []struct {
		now            time.Time
		timebitmap     int
		timedonebitmap int
		want           bool
	}{
		// morning planned, not done, checked at 13:00 -> missed
		{mk(13, 0), models.Treatement_MORNING, 0, true},
		// morning planned, done -> never missed
		{mk(13, 0), models.Treatement_MORNING, models.Treatement_MORNING, false},
		// morning planned, not done, checked at 11:00 -> not yet missed
		{mk(11, 0), models.Treatement_MORNING, 0, false},
		// evening planned, not done, checked at 13:00 -> not yet missed
		{mk(13, 0), models.Treatement_EVENING, 0, false},
		// noon planned, not done, checked at 18:00 -> missed
		{mk(18, 0), models.Treatement_NOON, 0, true},
		// nothing planned -> false
		{mk(13, 0), 0, 0, false},
	}
	for i, c := range cases {
		if got := guestSlotMissed(c.now, c.timebitmap, c.timedonebitmap); got != c.want {
			t.Errorf("case %d: guestSlotMissed(%s, %d, %d) = %v, want %v", i, c.now, c.timebitmap, c.timedonebitmap, got, c.want)
		}
	}
}

func TestGuestParseAnimalNumber(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in       string
		wantNum  int
		wantYear int
		wantOK   bool
	}{
		{"123", 123, 0, true},
		{"123/24", 123, 2024, true},
		{" 123 / 24 ", 123, 2024, true},
		{"abc", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		num, year, ok := guestParseAnimalNumber(c.in)
		if ok != c.wantOK || num != c.wantNum || year != c.wantYear {
			t.Errorf("guestParseAnimalNumber(%q) = (%d, %d, %v), want (%d, %d, %v)", c.in, num, year, ok, c.wantNum, c.wantYear, c.wantOK)
		}
	}
}

// ---------------------------------------------------------------------------
// guest.go: DB lookup + phone verification + outtake news
// (skips when the MySQL test database is unavailable)
// ---------------------------------------------------------------------------

type guestFixtures struct {
	marker       string
	animalID     int    // present in care, discoverer phone "06 12 34 56 78"
	animalGoneID int    // outtaken, outtake type has DiscovererNews
	newsText     string // DiscovererNews stored on the outtake type
	phone        string
}

func createGuestFixtures(t *testing.T, tx *pop.Connection) *guestFixtures {
	t.Helper()
	f := &guestFixtures{
		marker: uuid.Must(uuid.NewV4()).String()[:8],
		phone:  "06 12 34 56 78",
	}

	// unique yearNumber derived from marker (keeps clear of small production numbers)
	ynBase := 800000
	for _, b := range []byte(f.marker) {
		ynBase = ynBase*31 + int(b)
	}
	ynBase = 800000 + ynBase%90000

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("guest fixture creation failed: %v", err)
		}
	}

	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "GSType-" + f.marker}
	must(tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "GSAge-" + f.marker}
	must(tx.Create(&aa))
	ec := models.EntryCause{ID: "GSC-" + f.marker, Cause: "Cause " + f.marker, Detail: "d", Nature: "n", Indication: "i"}
	must(tx.Create(&ec))

	otNews := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "GSOut-" + f.marker, DiscovererNews: nulls.NewString("News-" + f.marker)}
	must(tx.Create(&otNews))

	mkDiscoverer := func() uuid.UUID {
		t.Helper()
		d := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("GS-" + f.marker), Phone: nulls.NewString(f.phone)}
		must(tx.Create(&d))
		return d.ID
	}
	mkAnimal := func(year, yearNumber int, outtaketypeID *uuid.UUID) int {
		t.Helper()
		discID := mkDiscoverer()
		d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: discID, EntryCauseID: ec.ID}
		must(tx.Create(&d))
		in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
		must(tx.Create(&in))
		var outtakeID nulls.UUID
		if outtaketypeID != nil {
			o := models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), TypeID: *outtaketypeID}
			must(tx.Create(&o))
			outtakeID = nulls.NewUUID(o.ID)
		}
		a := models.Animal{
			Year: year, YearNumber: yearNumber, Species: "Testsp Guest " + f.marker,
			AnimaltypeID: at.ID, AnimalageID: aa.ID,
			DiscoveryID: d.ID, IntakeID: in.ID, OuttakeID: outtakeID,
			IntakeDate: time.Now(),
		}
		must(tx.Create(&a))
		return a.ID
	}

	f.animalID = mkAnimal(time.Now().Year(), ynBase+1, nil)
	f.animalGoneID = mkAnimal(time.Now().Year(), ynBase+2, &otNews.ID)
	f.newsText = "News-" + f.marker

	t.Cleanup(func() {
		for _, id := range []int{f.animalID, f.animalGoneID} {
			tx.RawQuery("DELETE FROM animals WHERE id = ?", id).Exec()
		}
		tx.RawQuery("DELETE FROM outtakes WHERE outtaketype_id = ?", otNews.ID).Exec()
		tx.RawQuery("DELETE FROM discoveries WHERE entry_cause_id = ?", ec.ID).Exec()
		tx.RawQuery("DELETE FROM discoverers WHERE lastname = ?", "GS-"+f.marker).Exec()
		tx.RawQuery("DELETE FROM intakes WHERE id NOT IN (SELECT intake_id FROM animals)").Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
		tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec()
		tx.RawQuery("DELETE FROM entry_causes WHERE id = ?", ec.ID).Exec()
		tx.RawQuery("DELETE FROM outtaketypes WHERE id = ?", otNews.ID).Exec()
	})

	return f
}

// guestFixtureYearNumber reads the yearNumber column of a fixture animal.
func guestFixtureYearNumber(t *testing.T, tx *pop.Connection, id int) int {
	t.Helper()
	a := models.Animal{}
	if err := tx.Find(&a, id); err != nil {
		t.Fatalf("find fixture animal %d: %v", id, err)
	}
	return a.YearNumber
}

func TestGuestLookupAndVerify(t *testing.T) {
	tx := searchTestDB(t)
	f := createGuestFixtures(t, tx)
	yy := strconv.Itoa(time.Now().Year() % 100)

	// lookup present animal by "num/yy"
	yp := strconv.Itoa(guestFixtureYearNumber(t, tx, f.animalID))
	found, err := guestFindAnimal(tx, yp+"/"+yy)
	if err != nil || found == nil {
		t.Fatalf("guestFindAnimal present: err=%v found=%v", err, found)
	}
	if found.ID != f.animalID {
		t.Fatalf("guestFindAnimal returned id %d, want %d", found.ID, f.animalID)
	}

	// lookup without year suffix
	found2, err := guestFindAnimal(tx, yp)
	if err != nil || found2 == nil || found2.ID != f.animalID {
		t.Fatalf("guestFindAnimal by bare number: err=%v found=%v want id %d", err, found2, f.animalID)
	}

	// phone verification path
	d := models.Discovery{}
	if err := tx.Eager("Discoverer").Find(&d, found.DiscoveryID); err != nil {
		t.Fatalf("load discovery: %v", err)
	}
	if !guestPhoneMatches(d.Discoverer.Phone.String, "0612345678") {
		t.Errorf("phone should match stored %q", f.phone)
	}
	if guestPhoneMatches(d.Discoverer.Phone.String, "0611999999") {
		t.Error("phone should NOT match")
	}

	// gone animal: outtake news from outtake type
	yg := strconv.Itoa(guestFixtureYearNumber(t, tx, f.animalGoneID))
	gone, err := guestFindAnimal(tx, yg+"/"+yy)
	if err != nil || gone == nil {
		t.Fatalf("guestFindAnimal gone: err=%v found=%v", err, gone)
	}
	if !gone.OuttakeID.Valid {
		t.Fatal("gone animal should have an outtake")
	}
	o := models.Outtake{}
	if err := tx.Eager("Type").Find(&o, gone.OuttakeID.UUID); err != nil {
		t.Fatalf("load outtake: %v", err)
	}
	if o.Type.DiscovererNews.String != f.newsText {
		t.Errorf("DiscovererNews = %q, want %q", o.Type.DiscovererNews.String, f.newsText)
	}

	// unknown number -> nil, nil
	none, err := guestFindAnimal(tx, "999999/99")
	if err != nil || none != nil {
		t.Errorf("guestFindAnimal unknown = (%v, %v), want (nil, nil)", none, err)
	}
}

// ---------------------------------------------------------------------------
// QR code URL builder
// ---------------------------------------------------------------------------

func TestGuestStatusURL(t *testing.T) {
	got := guestStatusURL("https", "creaves.example.org", "1766/26",
		"abcd1234", "fr")
	want := "https://creaves.example.org/guest/?number=1766%2F26&token=abcd1234&lang=fr"
	if got != want {
		t.Errorf("guestStatusURL = %q, want %q", got, want)
	}

	got = guestStatusURL("http", "localhost:3000", "123", "", "")
	want = "http://localhost:3000/guest/?number=123"
	if got != want {
		t.Errorf("guestStatusURL plain = %q, want %q", got, want)
	}
}

func TestGuestPhoneToken(t *testing.T) {
	tok := guestPhoneToken("1766/26", "06 12 34 56 78")
	if len(tok) != 64 {
		t.Errorf("guestPhoneToken length = %d, want 64 (hex sha256)", len(tok))
	}
	// deterministic
	if again := guestPhoneToken("1766/26", "06 12 34 56 78"); again != tok {
		t.Errorf("guestPhoneToken not deterministic: %q != %q", again, tok)
	}
	// equivalent phone spellings may hash differently — the token is always
	// generated from the stored phone and compared to itself, so only
	// determinism matters, not cross-spelling equivalence.
	if eq := guestPhoneToken("1766/26", "0612345678"); eq != tok {
		t.Logf("note: spaced vs compact spelling hash differently (%q)", eq)
	}
	// different animal number (salt) => different token
	if diff := guestPhoneToken("1767/26", "06 12 34 56 78"); diff == tok {
		t.Error("guestPhoneToken ignores the animal number salt")
	}
	// different phone => different token
	if diff := guestPhoneToken("1766/26", "06 12 34 56 79"); diff == tok {
		t.Error("guestPhoneToken ignores the phone number")
	}
	// raw phone never appears in the token
	if strings.Contains(tok, "0612345678") {
		t.Error("guestPhoneToken leaks the raw phone number")
	}
}

func TestGuestTokenMatches(t *testing.T) {
	tok := guestPhoneToken("1766/26", "0612345678")
	if !guestTokenMatches(tok, tok) {
		t.Error("guestTokenMatches should accept the identical token")
	}
	if guestTokenMatches(tok, guestPhoneToken("1766/26", "0699999999")) {
		t.Error("guestTokenMatches accepted a wrong token")
	}
	if guestTokenMatches(tok, "") || guestTokenMatches("", tok) || guestTokenMatches("", "") {
		t.Error("guestTokenMatches must reject empty tokens")
	}
}

// TestGuestStoredPhoneToken exercises the AnimalQR direct-link logic against
// DB fixtures: the stored discoverer phone must produce the token accepted by
// the direct guest link.
func TestGuestStoredPhoneToken(t *testing.T) {
	tx := searchTestDB(t)
	f := createGuestFixtures(t, tx)
	a := &models.Animal{}
	if err := tx.Find(a, f.animalID); err != nil {
		t.Fatalf("load fixture animal: %v", err)
	}

	stored, err := guestStoredPhone(tx, a)
	if err != nil {
		t.Fatalf("guestStoredPhone: %v", err)
	}
	if stored == "" {
		t.Fatal("fixture discoverer phone should be set")
	}

	number := a.YearNumberFormatted()
	token := guestPhoneToken(number, stored)
	if !guestTokenMatches(token, token) {
		t.Fatal("roundtrip token mismatch")
	}
	if guestTokenMatches(token, guestPhoneToken(number, "wrong-phone")) {
		t.Error("token from wrong phone must not match")
	}

	// URL built by AnimalQR must open the view directly (token + lang)
	u := guestStatusURL("https", "creaves.example.org", number, token, "fr")
	if !strings.Contains(u, "token="+token) || !strings.Contains(u, "lang=fr") {
		t.Errorf("guestStatusURL missing token/lang: %q", u)
	}
}

func TestGuestScheme(t *testing.T) {
	if got := guestScheme(httptest.NewRequest("GET", "/x", nil)); got != "http" {
		t.Errorf("guestScheme plain = %q, want http", got)
	}
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := guestScheme(req); got != "https" {
		t.Errorf("guestScheme xfp = %q, want https", got)
	}
}
