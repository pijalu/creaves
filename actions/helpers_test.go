package actions

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// helper.go: sha256, timeToNullTime, timeToMinutes, switchTimeZone
// ---------------------------------------------------------------------------

func TestSha256(t *testing.T) {
	t.Parallel()
	// NOTE: despite its name, sha256 uses SHA-1 internally. We verify the
	// actual behaviour (deterministic SHA-1 hex digest), not the name.
	cases := []struct {
		in   string
		want string
	}{
		{"", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"hello", "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"creaves", "9ee85b9174132f2f952d46747c521789d44333d7"},
	}
	for _, c := range cases {
		got := sha256(c.in)
		if got != c.want {
			t.Errorf("sha256(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// Cross-check against an independent computation.
	h := sha1.Sum([]byte("cross-check"))
	if got, want := sha256("cross-check"), hex.EncodeToString(h[:]); got != want {
		t.Errorf("sha256 cross-check mismatch: got %q want %q", got, want)
	}
}

func TestTimeToNullTime(t *testing.T) {
	t.Parallel()
	// Valid time string
	got := timeToNullTime("13:45")
	if !got.Valid {
		t.Fatal("expected Valid time for valid input")
	}
	if got.Time.Hour() != 13 || got.Time.Minute() != 45 {
		t.Errorf("expected 13:45, got %02d:%02d", got.Time.Hour(), got.Time.Minute())
	}
	if got.Time.Location() != time.UTC {
		t.Errorf("expected UTC location, got %v", got.Time.Location())
	}

	// Invalid time string -> invalid nulls.Time
	invalid := timeToNullTime("not a time")
	if invalid.Valid {
		t.Error("expected invalid time for bad input")
	}

	// Empty string -> invalid
	empty := timeToNullTime("")
	if empty.Valid {
		t.Error("expected invalid time for empty input")
	}
}

func TestTimeToMinutes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int
	}{
		{"00:00", 0},
		{"01:00", 60},
		{"13:45", 13*60 + 45},
		{"23:59", 23*60 + 59},
		{"", 0},        // invalid -> 0
		{"abc", 0},     // invalid -> 0
		{"25:99", 0},   // invalid -> 0
	}
	for _, c := range cases {
		got := timeToMinutes(c.in)
		if got != c.want {
			t.Errorf("timeToMinutes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestSwitchTimeZone(t *testing.T) {
	t.Parallel()
	locUTC, _ := time.LoadLocation("UTC")
	locNY, _ := time.LoadLocation("America/New_York")

	original := time.Date(2024, 6, 15, 10, 30, 45, 123, locUTC)
	swapped := switchTimeZone(original, locNY)

	// Wall-clock components must be preserved.
	if swapped.Year() != 2024 || swapped.Month() != 6 || swapped.Day() != 15 {
		t.Errorf("date changed: got %d-%d-%d", swapped.Year(), swapped.Month(), swapped.Day())
	}
	if swapped.Hour() != 10 || swapped.Minute() != 30 || swapped.Second() != 45 {
		t.Errorf("time changed: got %02d:%02d:%02d", swapped.Hour(), swapped.Minute(), swapped.Second())
	}
	if swapped.Nanosecond() != 123 {
		t.Errorf("nanosecond changed: got %d", swapped.Nanosecond())
	}
	if swapped.Location() != locNY {
		t.Errorf("location not changed: got %v", swapped.Location())
	}
	// The absolute instant must differ (different zone, same wall-clock).
	if swapped.Equal(original) {
		t.Error("expected different instant after zone switch")
	}
}

// ---------------------------------------------------------------------------
// animals.go: containsUUID
// ---------------------------------------------------------------------------

func TestContainsUUID(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	id3 := uuid.Must(uuid.NewV4())

	tests := []struct {
		name  string
		slice []uuid.UUID
		id    uuid.UUID
		want  bool
	}{
		{"found first", []uuid.UUID{id1, id2}, id1, true},
		{"found last", []uuid.UUID{id1, id2}, id2, true},
		{"found middle", []uuid.UUID{id1, id2, id3}, id2, true},
		{"not found", []uuid.UUID{id1, id2}, id3, false},
		{"empty slice", []uuid.UUID{}, id1, false},
		{"nil slice", nil, id1, false},
		{"nil uuid not in non-empty", []uuid.UUID{id1}, uuid.Nil, false},
		{"nil uuid found", []uuid.UUID{id1, uuid.Nil}, uuid.Nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsUUID(tt.slice, tt.id); got != tt.want {
				t.Errorf("containsUUID = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// feeding.go: AnimalFeeding methods + calculateFeedings + FeedingByZoneMap.OrderedKeys
// ---------------------------------------------------------------------------

func TestAnimalFeedingString(t *testing.T) {
	t.Parallel()
	af := AnimalFeeding{
		ID:         5,
		Year:       2024,
		YearNumber: 3,
		Species:    "Hawk",
	}
	s := af.String()
	if !strings.Contains(s, `"ID":5`) {
		t.Errorf("String() missing ID: %s", s)
	}
	if !strings.Contains(s, `"Species":"Hawk"`) {
		t.Errorf("String() missing Species: %s", s)
	}
	// Should be valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Errorf("String() produced invalid JSON: %v (got %s)", err, s)
	}
}

func TestAnimalFeedingNextFeedingTime(t *testing.T) {
	t.Parallel()
	// Valid NextFeeding
	af := AnimalFeeding{
		NextFeeding: nulls.NewTime(time.Date(2024, 1, 2, 9, 5, 0, 0, time.UTC)),
	}
	if got, want := af.NextFeedingTime(), "09:05"; got != want {
		t.Errorf("NextFeedingTime() = %q, want %q", got, want)
	}
	// Invalid NextFeeding
	af2 := AnimalFeeding{NextFeeding: nulls.Time{}}
	if got, want := af2.NextFeedingTime(), "n.a."; got != want {
		t.Errorf("NextFeedingTime() invalid = %q, want %q", got, want)
	}
}

func TestAnimalFeedingNextFeedingFmt(t *testing.T) {
	t.Parallel()
	af := AnimalFeeding{
		NextFeeding: nulls.NewTime(time.Date(2024, 1, 2, 9, 5, 0, 0, time.UTC)),
	}
	if got, want := af.NextFeedingFmt(), "2024-01-02 09:05"; got != want {
		t.Errorf("NextFeedingFmt() = %q, want %q", got, want)
	}
	// Invalid -> empty string
	af2 := AnimalFeeding{NextFeeding: nulls.Time{}}
	if got := af2.NextFeedingFmt(); got != "" {
		t.Errorf("NextFeedingFmt() invalid = %q, want empty", got)
	}
}

func TestAnimalFeedingYearNumberFormatted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		year, num int
		want      string
	}{
		{2024, 3, "3/24"},
		{2000, 11, "11/0"},  // 2000 % 100 == 0
		{1999, 1, "1/99"},
		{2010, 42, "42/10"},
	}
	for _, c := range cases {
		af := AnimalFeeding{Year: c.year, YearNumber: c.num}
		if got := af.YearNumberFormatted(); got != c.want {
			t.Errorf("YearNumberFormatted(%d,%d) = %q, want %q", c.year, c.num, got, c.want)
		}
	}
}

func TestFeedingByZoneMapOrderedKeys(t *testing.T) {
	t.Parallel()
	m := FeedingByZoneMap{
		{ID: "c", Name: "Quarantine"}: {},
		{ID: "a", Name: "Aviary"}:     {},
		{ID: "b", Name: "Bunker"}:     {},
	}
	keys := m.OrderedKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	// Keys must be sorted by Name ascending.
	want := []string{"Aviary", "Bunker", "Quarantine"}
	for i, k := range keys {
		if k.Name != want[i] {
			t.Errorf("keys[%d].Name = %q, want %q", i, k.Name, want[i])
		}
	}
}

func TestFeedingByZoneMapOrderedKeysEmpty(t *testing.T) {
	t.Parallel()
	m := FeedingByZoneMap{}
	keys := m.OrderedKeys()
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for empty map, got %d", len(keys))
	}
}

func TestCalculateFeedingsEmpty(t *testing.T) {
	t.Parallel()
	res, err := calculateFeedings([]AnimalFeeding{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty map, got %d keys", len(res))
	}
}

func TestCalculateFeedingsNil(t *testing.T) {
	t.Parallel()
	res, err := calculateFeedings(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected empty map for nil input, got %d keys", len(res))
	}
}

// makeAF builds an AnimalFeeding with a wide daily feeding window so that
// calculateFeeding is very likely to produce a valid NextFeeding regardless
// of the wall-clock time at which the test runs.
func makeAF(id int, zone string) AnimalFeeding {
	af := AnimalFeeding{
		ID:            id,
		Year:          2024,
		YearNumber:    1,
		Species:       "Test",
		Feeding:       "daily",
		ForceFeed:     false,
		FeedingPeriod: 30,
	}
	if zone != "" {
		af.Zone = nulls.NewString(zone)
	}
	// Wide window covering almost the entire day.
	startTime, _ := time.ParseInLocation("15:04", "00:05", time.Local)
	endTime, _ := time.ParseInLocation("15:04", "23:55", time.Local)
	af.FeedingStart = startTime
	af.FeedingEnd = endTime
	return af
}

func TestCalculateFeedingsGroupingAndSorting(t *testing.T) {
	t.Parallel()
	// Build two animals in the same zone and one in a different zone.
	af1 := makeAF(1, "ZoneA")
	af2 := makeAF(2, "ZoneA")
	af3 := makeAF(3, "ZoneB")
	// One animal with no zone -> should land under the "?" key.
	af4 := makeAF(4, "")

	res, err := calculateFeedings([]AnimalFeeding{af1, af2, af3, af4})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All returned entries must be grouped by zone key, and within a zone
	// the entries must be sorted ascending by NextFeeding time.
	for key, group := range res {
		if len(group) == 0 {
			t.Errorf("key %v has empty group", key)
			continue
		}
		// Verify the key matches the zone (or "?" for empty zone).
		for _, f := range group {
			if f.Zone.Valid {
				if f.Zone.String != key.Name {
					t.Errorf("entry zone %q does not match key name %q", f.Zone.String, key.Name)
				}
			} else {
				if key.Name != "?" {
					t.Errorf("zoneless entry grouped under %q, want ?", key.Name)
				}
			}
		}
		// Verify sorting within group.
		for i := 1; i < len(group); i++ {
			if group[i].NextFeeding.Time.Before(group[i-1].NextFeeding.Time) {
				t.Errorf("group %q not sorted at index %d", key.Name, i)
			}
		}
	}

	// If all four produced valid NextFeedings, we expect exactly 3 keys
	// (ZoneA, ZoneB, "?"). Because calculateFeeding depends on time.Now(),
	// some entries might be filtered out in a narrow edge window, so we
	// only assert an upper bound and correct grouping.
	if len(res) > 3 {
		t.Errorf("expected at most 3 zone keys, got %d", len(res))
	}

	// ZoneA should have both animals grouped together (when valid).
	if za, ok := lookupZone(res, "ZoneA"); ok && len(za) == 2 {
		// verify IDs present
		seen := map[int]bool{}
		for _, f := range za {
			seen[f.ID] = true
		}
		if !seen[1] || !seen[2] {
			t.Errorf("ZoneA group missing expected IDs, got %v", seen)
		}
	}
}

// lookupZone finds the group for a given zone name.
func lookupZone(m FeedingByZoneMap, name string) ([]AnimalFeeding, bool) {
	for k, v := range m {
		if k.Name == name {
			return v, true
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// typehelper.go: selType, *ToSelectables, usersToMap, selectFeedingPeriod, BoolToInt
// ---------------------------------------------------------------------------

func TestSelType(t *testing.T) {
	t.Parallel()
	st := &selType{label: "My Label", value: "myval"}
	if got := st.SelectLabel(); got != "My Label" {
		t.Errorf("SelectLabel() = %q, want %q", got, "My Label")
	}
	if got := st.SelectValue(); got != "myval" {
		t.Errorf("SelectValue() = %v, want %v", got, "myval")
	}
}

func TestBoolToInt(t *testing.T) {
	t.Parallel()
	if BoolToInt(true) != 1 {
		t.Error("BoolToInt(true) should be 1")
	}
	if BoolToInt(false) != 0 {
		t.Error("BoolToInt(false) should be 0")
	}
}

func TestAnimalTypesToSelectables_NoDefault(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	ts := &models.Animaltypes{
		{ID: id1, Name: "Bird"},
		{ID: id2, Name: "Mammal"},
	}
	res := animalTypesToSelectables(ts, "", nil)
	// No default => blank entry kept as first element.
	if len(res) != 3 {
		t.Fatalf("expected 3 selectables (blank + 2), got %d", len(res))
	}
	if res[0].SelectLabel() != "" {
		t.Errorf("first entry should be blank, got %q", res[0].SelectLabel())
	}
	if res[1].SelectLabel() != "Bird" {
		t.Errorf("second entry label = %q, want Bird", res[1].SelectLabel())
	}
	if res[1].SelectValue() != id1 {
		t.Errorf("second entry value = %v, want %v", res[1].SelectValue(), id1)
	}
}

func TestAnimalTypesToSelectables_WithDefault(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	ts := &models.Animaltypes{
		{ID: id1, Name: "Bird"},
		{ID: id2, Name: "Mammal", Default: true},
	}
	res := animalTypesToSelectables(ts, "", nil)
	// Default present => blank entry removed.
	if len(res) != 2 {
		t.Fatalf("expected 2 selectables (blank removed), got %d", len(res))
	}
	for _, s := range res {
		if s.SelectLabel() == "" {
			t.Error("blank entry should have been removed when default exists")
		}
	}
}

func TestAnimalTypesToSelectables_Empty(t *testing.T) {
	t.Parallel()
	res := animalTypesToSelectables(&models.Animaltypes{}, "", nil)
	if len(res) != 1 {
		t.Fatalf("expected 1 (blank only), got %d", len(res))
	}
}

func TestZonesToSelectables(t *testing.T) {
	t.Parallel()
	zs := &models.Zones{
		{Zone: "Aviary", Type: "bird"},
		{Zone: "Tank", Type: "fish"},
	}
	res := zonesToSelectables(zs, "", nil)
	if len(res) != 3 { // blank + 2
		t.Fatalf("expected 3 selectables, got %d", len(res))
	}
	if res[0].SelectLabel() != "" {
		t.Errorf("first should be blank, got %q", res[0].SelectLabel())
	}
	// Zone value is the zone name string.
	if res[1].SelectLabel() != "Aviary" || res[1].SelectValue() != "Aviary" {
		t.Errorf("zone entry = label %q value %v", res[1].SelectLabel(), res[1].SelectValue())
	}
}

func TestOuttakeTypesToSelectables(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	ots := &models.Outtaketypes{
		{ID: id1, Name: "Release"},
		{ID: id2, Name: "Death"},
	}
	res := outtakeTypesToSelectables(ots, "", nil)
	if len(res) != 2 { // no blank for outtake types
		t.Fatalf("expected 2 selectables, got %d", len(res))
	}
	if res[0].SelectLabel() != "Release" || res[0].SelectValue() != id1 {
		t.Errorf("entry 0 = label %q value %v", res[0].SelectLabel(), res[0].SelectValue())
	}
}

func TestCaretypesToSelectables(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	cts := &models.Caretypes{
		{ID: id1, Name: "Feeding"},
	}
	res := caretypesToSelectables(cts, "", nil)
	if len(res) != 1 {
		t.Fatalf("expected 1 selectable, got %d", len(res))
	}
	if res[0].SelectLabel() != "Feeding" || res[0].SelectValue() != id1 {
		t.Errorf("entry = label %q value %v", res[0].SelectLabel(), res[0].SelectValue())
	}
}

func TestTraveltypesToSelectables(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	tts := &models.Traveltypes{
		{ID: id1, Name: "Transport"},
	}
	res := traveltypesToSelectables(tts, "", nil)
	if len(res) != 1 {
		t.Fatalf("expected 1 selectable, got %d", len(res))
	}
	if res[0].SelectLabel() != "Transport" {
		t.Errorf("label = %q", res[0].SelectLabel())
	}
}

func TestAnimalagesToSelectables(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	aas := &models.Animalages{
		{ID: id1, Name: "Adult"},
	}
	res := animalagesToSelectables(aas, "", nil)
	if len(res) != 1 {
		t.Fatalf("expected 1 selectable, got %d", len(res))
	}
	if res[0].SelectLabel() != "Adult" || res[0].SelectValue() != id1 {
		t.Errorf("entry = label %q value %v", res[0].SelectLabel(), res[0].SelectValue())
	}
}

func TestUsersToMap(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	us := &models.Users{
		{ID: id1, Login: "alice"},
		{ID: id2, Login: "bob"},
	}
	m := usersToMap(us)
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m))
	}
	if m[id1].Login != "alice" {
		t.Errorf("id1 login = %q, want alice", m[id1].Login)
	}
	if m[id2].Login != "bob" {
		t.Errorf("id2 login = %q, want bob", m[id2].Login)
	}
}

func TestUsersToMap_Empty(t *testing.T) {
	t.Parallel()
	m := usersToMap(&models.Users{})
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d", len(m))
	}
}

func TestUsersToSelectables(t *testing.T) {
	t.Parallel()
	id1 := uuid.Must(uuid.NewV4())
	us := &models.Users{
		{ID: id1, Login: "alice"},
	}
	res := usersToSelectables(us)
	if len(res) != 1 {
		t.Fatalf("expected 1 selectable, got %d", len(res))
	}
	if res[0].SelectLabel() != "alice" || res[0].SelectValue() != id1 {
		t.Errorf("entry = label %q value %v", res[0].SelectLabel(), res[0].SelectValue())
	}
}

func TestSelectFeedingPeriod(t *testing.T) {
	t.Parallel()
	res := selectFeedingPeriod()
	// N.A. (0), 15min (15), 30min (30), then 1h..12h => 3 + 12 = 15
	if len(res) != 15 {
		t.Fatalf("expected 15 selectables, got %d", len(res))
	}
	// First entry is N.A. with value 0.
	if res[0].SelectLabel() != "N.A." || res[0].SelectValue() != 0 {
		t.Errorf("first entry = label %q value %v", res[0].SelectLabel(), res[0].SelectValue())
	}
	// Verify 15min and 30min.
	if res[1].SelectLabel() != "15min" || res[1].SelectValue() != 15 {
		t.Errorf("second entry = label %q value %v", res[1].SelectLabel(), res[1].SelectValue())
	}
	if res[2].SelectLabel() != "30min" || res[2].SelectValue() != 30 {
		t.Errorf("third entry = label %q value %v", res[2].SelectLabel(), res[2].SelectValue())
	}
	// Verify hourly entries 1h..12h.
	for i := 1; i <= 12; i++ {
		idx := 2 + i
		wantLabel := strings.Replace("Xh", "X", intTostr(i), 1)
		if res[idx].SelectLabel() != wantLabel {
			t.Errorf("entry %d label = %q, want %q", idx, res[idx].SelectLabel(), wantLabel)
		}
		if res[idx].SelectValue() != i*60 {
			t.Errorf("entry %d value = %v, want %d", idx, res[idx].SelectValue(), i*60)
		}
	}
}

// intTostr is a tiny local helper to avoid importing strconv just for this.
func intTostr(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestEntryCausesToSelectables(t *testing.T) {
	t.Parallel()
	id1 := "ec1"
	id2 := "ec2"
	ecs := &models.EntryCauses{
		{ID: id1, Cause: "Found", Detail: "Found"},
		{ID: id2, Cause: "Injury", Detail: "Broken wing"},
	}

	// With blank — base lang, no tx (canonical French fallback path).
	res := entryCausesToSelectables(ecs, true, "", nil)
	if len(res) != 3 { // blank + 2
		t.Fatalf("expected 3 selectables with blank, got %d", len(res))
	}
	if res[0].SelectLabel() != " " {
		t.Errorf("first should be blank space, got %q", res[0].SelectLabel())
	}
	if res[1].SelectValue() != id1 {
		t.Errorf("entry 1 value = %v, want %s", res[1].SelectValue(), id1)
	}
	// entryCausesToSelectables always formats with ID prefix (Fmt(true)).
	if res[1].SelectLabel() != "ec1 - Found" {
		t.Errorf("entry 1 label = %q, want 'ec1 - Found'", res[1].SelectLabel())
	}

	// Without blank
	res2 := entryCausesToSelectables(ecs, false, "", nil)
	if len(res2) != 2 {
		t.Fatalf("expected 2 selectables without blank, got %d", len(res2))
	}
	// Cause == Detail -> "ID - Cause"
	if res2[0].SelectLabel() != "ec1 - Found" {
		t.Errorf("entry 0 label = %q, want 'ec1 - Found'", res2[0].SelectLabel())
	}
	// Cause != Detail -> "ID - Cause ⇨ Detail"
	if res2[1].SelectLabel() != "ec2 - Injury ⇨ Broken wing" {
		t.Errorf("entry 1 label = %q, want 'ec2 - Injury ⇨ Broken wing'", res2[1].SelectLabel())
	}
}

func TestEntryCauseLabel_Translated(t *testing.T) {
	t.Parallel()
	ec := models.EntryCause{ID: "ec9", Cause: "Blessure", Detail: "Aile cassée"}

	// No translation maps -> canonical French.
	if got := entryCauseLabel(ec, "en-US", nil, nil); got != "ec9 - Blessure ⇨ Aile cassée" {
		t.Errorf("untranslated label = %q", got)
	}

	// Translated cause and detail keep the "ID - cause ⇨ detail" format.
	causeTr := map[string]string{"ec9": "Injury"}
	detailTr := map[string]string{"ec9": "Broken wing"}
	if got := entryCauseLabel(ec, "en-US", causeTr, detailTr); got != "ec9 - Injury ⇨ Broken wing" {
		t.Errorf("translated label = %q, want 'ec9 - Injury ⇨ Broken wing'", got)
	}

	// Detail translation missing -> canonical detail, translated cause.
	if got := entryCauseLabel(ec, "en-US", causeTr, nil); got != "ec9 - Injury ⇨ Aile cassée" {
		t.Errorf("partial label = %q", got)
	}

	// Cause == detail (both translated to same) -> "ID - cause" only.
	same := models.EntryCause{ID: "ec1", Cause: "Indéterminé", Detail: "Indéterminé"}
	if got := entryCauseLabel(same, "en-US", map[string]string{"ec1": "Unknown"}, map[string]string{"ec1": "Unknown"}); got != "ec1 - Unknown" {
		t.Errorf("equal cause/detail label = %q, want 'ec1 - Unknown'", got)
	}

	// Base lang (fr) ignores maps.
	if got := entryCauseLabel(ec, "", causeTr, detailTr); got != "ec9 - Blessure ⇨ Aile cassée" {
		t.Errorf("base-lang label = %q", got)
	}
}

func TestHintLocalizeSpeciesHints(t *testing.T) {
	t.Parallel()

	// Base lang: no-op even with maps present.
	rows := []speciesHint{
		{ID: "ns1", Status: "Espèce protégée", Indication: "Ne pas toucher"},
	}
	localizeSpeciesHints("", map[string]string{"ns1": "Protected"}, nil, nil, rows)
	if rows[0].Status != "Espèce protégée" || rows[0].Indication != "Ne pas toucher" {
		t.Errorf("base lang rows modified: %+v", rows[0])
	}

	// Translated status/indication; precision untouched when invalid.
	rows = []speciesHint{
		{ID: "ns1", Status: "Espèce protégée", Indication: "Ne pas toucher", Precision: nulls.NewString("Précision FR")},
		{ID: "ns2", Status: "Sans statut", Indication: "Canonique", Precision: nulls.String{}},
	}
	statusTr := map[string]string{"ns1": "Protected species"}
	indicationTr := map[string]string{"ns1": "Do not touch", "ns2": "Translated without row"}
	precisionTr := map[string]string{"ns1": "Precision EN"}
	localizeSpeciesHints("de", statusTr, indicationTr, precisionTr, rows)

	if rows[0].Status != "Protected species" {
		t.Errorf("status = %q, want 'Protected species'", rows[0].Status)
	}
	if rows[0].Indication != "Do not touch" {
		t.Errorf("indication = %q, want 'Do not touch'", rows[0].Indication)
	}
	if rows[0].Precision.String != "Precision EN" {
		t.Errorf("precision = %q, want 'Precision EN'", rows[0].Precision.String)
	}
	// ns2 has no status translation -> canonical fallback; indication
	// translated even though its status is not.
	if rows[1].Status != "Sans statut" {
		t.Errorf("fallback status = %q, want 'Sans statut'", rows[1].Status)
	}
	if rows[1].Indication != "Translated without row" {
		t.Errorf("fallback indication = %q", rows[1].Indication)
	}
	// Invalid precision stays invalid/empty.
	if rows[1].Precision.Valid {
		t.Errorf("invalid precision mutated: %+v", rows[1].Precision)
	}
}

// ---------------------------------------------------------------------------
// treatments.go: treatmentSchedule.String()
// ---------------------------------------------------------------------------

func TestTreatmentScheduleString(t *testing.T) {
	t.Parallel()
	ts := &treatmentSchedule{
		ScheduleRequiredMorning: nulls.NewBool(true),
		ScheduleRequiredNoon:    nulls.NewBool(false),
		ScheduleRequiredEvening: nulls.Bool{},
		ScheduleStatusMorning:   nulls.NewBool(true),
	}
	s := ts.String()
	if !strings.Contains(s, "ScheduleRequiredMorning") {
		t.Errorf("String() missing field: %s", s)
	}
	// Must be valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Errorf("String() produced invalid JSON: %v (got %s)", err, s)
	}
}

func TestTreatmentScheduleString_Nil(t *testing.T) {
	t.Parallel()
	var ts *treatmentSchedule
	// json.Marshal on nil pointer returns "null", no panic.
	s := ts.String()
	if s != "null" {
		t.Errorf("nil String() = %q, want null", s)
	}
}

// ---------------------------------------------------------------------------
// cache_utils.go: InvalidateWeightLossCache (pure state reset under mutex)
// ---------------------------------------------------------------------------

func TestInvalidateWeightLossCache(t *testing.T) {
	// Set a non-zero last update to simulate a populated cache.
	cacheMutex.Lock()
	cacheLastUpdate = time.Now()
	cacheMutex.Unlock()

	InvalidateWeightLossCache()

	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	if !cacheLastUpdate.IsZero() {
		t.Errorf("expected cacheLastUpdate to be zero after invalidation, got %v", cacheLastUpdate)
	}
}

func TestInvalidateWeightLossCache_Idempotent(t *testing.T) {
	// Calling invalidate on an already-zero cache should remain zero.
	cacheMutex.Lock()
	cacheLastUpdate = time.Time{}
	cacheMutex.Unlock()

	InvalidateWeightLossCache()

	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	if !cacheLastUpdate.IsZero() {
		t.Errorf("expected cacheLastUpdate to stay zero, got %v", cacheLastUpdate)
	}
}

// refreshWeightLossCache is a no-op in the current implementation; we verify
// it does not panic and returns without error.
func TestRefreshWeightLossCache_Noop(t *testing.T) {
	refreshWeightLossCache() // must not panic
}
