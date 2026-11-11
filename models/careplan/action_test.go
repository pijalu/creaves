package careplan

import (
	"strings"
	"testing"
)

// §4.2 — action_payload validation per kind (schema parity rules/animal
// plans §4.7), strict unknown keys, optional `instructions` on every
// payload (EN3).
func TestValidateActionKind(t *testing.T) {
	for _, k := range []string{KindFeeding, KindMedication, KindCare, KindCleanup, KindWeighing, KindObservation} {
		if err := ValidateActionKind(k); err != nil {
			t.Errorf("ValidateActionKind(%q): %v", k, err)
		}
	}
	if err := ValidateActionKind("massage"); err == nil {
		t.Errorf("unknown kind must be rejected")
	}
	if err := ValidateActionKind(""); err == nil {
		t.Errorf("empty kind must be rejected")
	}
}

func TestValidateActionPayloadValidExamples(t *testing.T) {
	// The §4.2 example documents, one per kind.
	for kind, raw := range map[string]string{
		KindFeeding:     `{"caretype_id":"ct-1","food":"Croquettes + 4 VDF + EAU","force_feed":false,"note":"…"}`,
		KindMedication:  `{"drug":"Ivomec 1% (SC)","dosage":"0.1 ml / 100g","remarks":"…"}`,
		KindCare:        `{"caretype_id":"ct-2","note":"Changer bandage","heat_source_check":true}`,
		KindCleanup:     `{"note":"Nettoyage cage"}`,
		KindWeighing:    `{"note":"…"}`,
		KindObservation: `{"prompt":"Mange seul ?","note":"…","alert_on":"no","alert_follow_up_hours":4}`,
		// dosage via dosages-table resolution path (§4.2/B6)
		KindMedication + "2": `{"drug":"Ivomec 1% (SC)","dosage_from_dosages_table":true}`,
		// minimal shapes
		KindCleanup + "2":  `{}`,
		KindWeighing + "2": `{}`,
		// EN3: instructions on any payload
		KindFeeding + "3": `{"caretype_id":"ct-1","instructions":"Gavage technique (photos)"}`,
	} {
		if err := ValidateActionPayload(kindRawKind(kind), []byte(raw)); err != nil {
			t.Errorf("%s: ValidateActionPayload(%s): %v", kind, raw, err)
		}
	}
}

func kindRawKind(k string) string {
	if i := strings.Index(k, "2"); i > 0 {
		return k[:i]
	}
	if i := strings.Index(k, "3"); i > 0 {
		return k[:i]
	}
	return k
}

func TestValidateActionPayloadRequiredFields(t *testing.T) {
	cases := []struct{ kind, raw, wantMention string }{
		{KindFeeding, `{}`, "caretype_id"},
		{KindFeeding, `{"food":"x"}`, "caretype_id"},
		{KindFeeding, `{"caretype_id":""}`, "caretype_id"},
		{KindCare, `{}`, "caretype_id"},
		{KindMedication, `{"drug":"Ivomec"}`, "dosage"},             // no resolution path
		{KindMedication, `{"drug":"Ivomec","dosage":""}`, "dosage"}, // empty literal ≠ path
		{KindObservation, `{}`, "prompt"},
		{KindObservation, `{"prompt":""}`, "prompt"},
	}
	for _, c := range cases {
		err := ValidateActionPayload(c.kind, []byte(c.raw))
		if err == nil {
			t.Errorf("%s %s: expected error mentioning %q", c.kind, c.raw, c.wantMention)
		} else if !strings.Contains(err.Error(), c.wantMention) {
			t.Errorf("%s %s: error %v must mention %q", c.kind, c.raw, err, c.wantMention)
		}
	}
}

func TestValidateActionPayloadMedicationResolutionPaths(t *testing.T) {
	// Both paths at once is ambiguous — rejected (§4.2 "literal, or:").
	err := ValidateActionPayload(KindMedication, []byte(`{"drug":"X","dosage":"0.1ml","dosage_from_dosages_table":true}`))
	if err == nil || !strings.Contains(err.Error(), "dosage") {
		t.Errorf("both dosage paths must be rejected, got %v", err)
	}
	// dosage_from_dosages_table must be a boolean.
	err = ValidateActionPayload(KindMedication, []byte(`{"drug":"X","dosage_from_dosages_table":"yes"}`))
	if err == nil {
		t.Errorf("non-bool dosage_from_dosages_table must be rejected")
	}
}

func TestValidateActionPayloadObservationAlert(t *testing.T) {
	for _, bad := range []string{
		`{"prompt":"?","alert_on":"maybe"}`,
		`{"prompt":"?","alert_on":1}`,
		`{"prompt":"?","alert_follow_up_hours":0}`,
		`{"prompt":"?","alert_follow_up_hours":-4}`,
	} {
		if err := ValidateActionPayload(KindObservation, []byte(bad)); err == nil {
			t.Errorf("%s must be rejected", bad)
		}
	}
	// alert_on null = no alert (§4.2: "no" | "yes" | null).
	if err := ValidateActionPayload(KindObservation, []byte(`{"prompt":"?","alert_on":null}`)); err != nil {
		t.Errorf("alert_on null must be accepted: %v", err)
	}
	if err := ValidateActionPayload(KindObservation, []byte(`{"prompt":"?","alert_on":"yes"}`)); err != nil {
		t.Errorf("alert_on yes must be accepted: %v", err)
	}
}

func TestValidateActionPayloadStrictness(t *testing.T) {
	cases := []string{
		`{"caretype_id":"ct","foodk":"typo"}`, // unknown key feeding
		`{"note":"x","clean":1}`,              // unknown key cleanup
		`{"prompt":"?","answer":"42"}`,        // unknown key observation (answer comes at apply, §6.2)
		`not json`,
		`[]`,                                     // not an object
		`"weigh"`,                                // not an object
		`null`,                                   // not an object
		`{"note":"a"}{"note":"b"}`,               // trailing data
		`{"instructions":42,"caretype_id":"ct"}`, // instructions must be a string
	}
	for _, raw := range cases {
		kind := KindFeeding
		if strings.Contains(raw, `"note"`) && !strings.Contains(raw, "caretype_id") {
			kind = KindCleanup
		}
		if strings.Contains(raw, "prompt") || strings.Contains(raw, "instructions") && strings.Contains(raw, "answer") {
			kind = KindObservation
		}
		if raw == `{"instructions":42,"caretype_id":"ct"}` {
			kind = KindFeeding
		}
		if err := ValidateActionPayload(kind, []byte(raw)); err == nil {
			t.Errorf("%s %s must be rejected", kind, raw)
		}
	}
	// Unknown kind is rejected too.
	if err := ValidateActionPayload("massage", []byte(`{}`)); err == nil {
		t.Errorf("unknown kind must be rejected")
	}
}
