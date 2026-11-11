package careplan

import (
	"strings"
	"testing"
)

// BUG-45: the per-tier display caps round-trip through the §4.3 document;
// 0/absent = uncapped; negatives clamp to 0 (never rejected — UX-only).
func TestDisplayMaxCapsRoundTrip(t *testing.T) {
	raw := []byte(`{"times":["08:00","12:00"],"display_max_late":2,"display_max_now":5,"display_max_future":3}`)
	s, err := ParseScheduleJSON(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.DisplayMaxLate != 2 || s.DisplayMaxNow != 5 || s.DisplayMaxFuture != 3 {
		t.Fatalf("caps = %d/%d/%d, want 2/5/3", s.DisplayMaxLate, s.DisplayMaxNow, s.DisplayMaxFuture)
	}
	canon, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{`"display_max_late":2`, `"display_max_now":5`, `"display_max_future":3`} {
		if !strings.Contains(string(canon), key) {
			t.Errorf("canonical doc %s missing %s", canon, key)
		}
	}
	// Re-parse the canonical doc: same values.
	s2, err := ParseScheduleJSON(canon)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if s2.DisplayMaxLate != 2 || s2.DisplayMaxNow != 5 || s2.DisplayMaxFuture != 3 {
		t.Fatalf("round-trip caps = %d/%d/%d, want 2/5/3", s2.DisplayMaxLate, s2.DisplayMaxNow, s2.DisplayMaxFuture)
	}
}

func TestDisplayMaxCapsDefaultsAndClamp(t *testing.T) {
	// Absent: all zero (uncapped) and omitted from the canonical doc.
	s, err := ParseScheduleJSON([]byte(`{"times":["08:00"]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.DisplayMaxLate != 0 || s.DisplayMaxNow != 0 || s.DisplayMaxFuture != 0 {
		t.Fatalf("default caps = %d/%d/%d, want 0/0/0", s.DisplayMaxLate, s.DisplayMaxNow, s.DisplayMaxFuture)
	}
	canon, _ := s.MarshalJSON()
	if strings.Contains(string(canon), "display_max") {
		t.Errorf("canonical doc %s must omit zero caps", canon)
	}

	// Negative values clamp to 0 (uncapped), never an error.
	s, err = ParseScheduleJSON([]byte(`{"times":["08:00"],"display_max_late":-3,"display_max_now":0}`))
	if err != nil {
		t.Fatalf("parse negative: %v", err)
	}
	if s.DisplayMaxLate != 0 || s.DisplayMaxNow != 0 {
		t.Fatalf("clamped caps = %d/%d, want 0/0", s.DisplayMaxLate, s.DisplayMaxNow)
	}

	// NormalizeScheduleJSON preserves the caps.
	norm, err := NormalizeScheduleJSON([]byte(`{"times":["08:00"],"display_max_future":4}`))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !strings.Contains(string(norm), `"display_max_future":4`) {
		t.Errorf("normalized doc %s lost the cap", norm)
	}
}
