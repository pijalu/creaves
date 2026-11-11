package careplan

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Action payload validation (§4.2, §6.2 step L1): the payload schema is
// validated per action_kind at save time (rule editor AND animal-plan
// editor — full parity, §4.7). Purely structural: FK-ish references
// (caretype_id, feeding-caretype check) and runtime resolution (dosages
// table, §10-B6) belong to the model/service layer, which has DB access.
// Strict like the schedule (§4.3): unknown keys are typos, not forward
// compatibility.

// Action kinds (§4.1/§4.2, extensible in code, validated in Go).
const (
	KindFeeding     = "feeding"
	KindMedication  = "medication"
	KindCare        = "care"
	KindCleanup     = "cleanup"
	KindWeighing    = "weighing"
	KindObservation = "observation"
)

// ValidateActionKind checks the action_kind discriminates a known kind.
func ValidateActionKind(kind string) error {
	switch kind {
	case KindFeeding, KindMedication, KindCare, KindCleanup, KindWeighing, KindObservation:
		return nil
	case "":
		return fmt.Errorf("careplan: action_kind is required")
	default:
		return fmt.Errorf("careplan: unknown action_kind %q (want feeding|medication|care|cleanup|weighing|observation)", kind)
	}
}

// payloadBase carries the keys shared by every kind: instructions (EN3).
type payloadBase struct {
	Instructions string `json:"instructions"`
	Note         string `json:"note"`
}

type feedingPayload struct {
	payloadBase
	CaretypeID string `json:"caretype_id"` // required, feeding-type caretype
	Food       string `json:"food"`
	ForceFeed  bool   `json:"force_feed"`
}

type medicationPayload struct {
	payloadBase
	Drug   string `json:"drug"` // required
	Dosage string `json:"dosage"`
	// DosageFromTable: resolve via drugs.dosages × animaltype × LastWeight.
	DosageFromTable *bool  `json:"dosage_from_dosages_table"`
	Remarks         string `json:"remarks"`
}

type carePayload struct {
	payloadBase
	CaretypeID      string `json:"caretype_id"` // required
	HeatSourceCheck bool   `json:"heat_source_check"`
}

type observationPayload struct {
	payloadBase
	Prompt string `json:"prompt"` // required: the apply-time question
	// AlertOn: "no" | "yes" | null (§10.1-6: alert is v1).
	AlertOn *string `json:"alert_on"`
	// AlertFollowUpHours: follow-up plan due now+N h (§10-CP3).
	AlertFollowUpHours *int `json:"alert_follow_up_hours"`
}

// ValidateActionPayload validates one §4.2 payload document for its kind.
func ValidateActionPayload(kind string, raw []byte) error {
	if err := ValidateActionKind(kind); err != nil {
		return err
	}
	switch kind {
	case KindFeeding:
		var p feedingPayload
		if err := decodePayload(raw, &p); err != nil {
			return err
		}
		if p.CaretypeID == "" {
			return fmt.Errorf("careplan: feeding payload needs caretype_id (§4.2)")
		}
	case KindMedication:
		var p medicationPayload
		if err := decodePayload(raw, &p); err != nil {
			return err
		}
		if p.Drug == "" {
			return fmt.Errorf("careplan: medication payload needs drug (§4.2)")
		}
		hasLiteral := p.Dosage != ""
		hasTable := p.DosageFromTable != nil && *p.DosageFromTable
		switch {
		case hasLiteral && hasTable:
			return fmt.Errorf("careplan: medication payload: choose ONE dosage path — dosage or dosage_from_dosages_table (§4.2)")
		case !hasLiteral && !hasTable:
			return fmt.Errorf("careplan: medication payload needs dosage or dosage_from_dosages_table (§4.2)")
		}
	case KindCare:
		var p carePayload
		if err := decodePayload(raw, &p); err != nil {
			return err
		}
		if p.CaretypeID == "" {
			return fmt.Errorf("careplan: care payload needs caretype_id (§4.2)")
		}
	case KindCleanup, KindWeighing:
		// No required fields: cleanup fulfills via cares.clean=1, weighing
		// via a care row carrying a weight (§4.2).
		var p payloadBase
		if err := decodePayload(raw, &p); err != nil {
			return err
		}
	case KindObservation:
		var p observationPayload
		if err := decodePayload(raw, &p); err != nil {
			return err
		}
		if p.Prompt == "" {
			return fmt.Errorf("careplan: observation payload needs prompt (§4.2)")
		}
		if p.AlertOn != nil && *p.AlertOn != "no" && *p.AlertOn != "yes" {
			return fmt.Errorf(`careplan: observation payload: alert_on must be "no", "yes" or null (§4.2), got %q`, *p.AlertOn)
		}
		if p.AlertFollowUpHours != nil && *p.AlertFollowUpHours < 1 {
			return fmt.Errorf("careplan: observation payload: alert_follow_up_hours must be ≥ 1 (§10-CP3), got %d", *p.AlertFollowUpHours)
		}
	}
	return nil
}

// decodePayload strictly decodes one payload document: must be a JSON
// object, unknown keys rejected, no trailing data (same strictness as the
// schedule parser, §4.3).
func decodePayload(raw []byte, v any) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return fmt.Errorf("careplan: action payload must be a JSON object")
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("careplan: invalid action payload: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("careplan: invalid action payload: trailing data")
	}
	return nil
}
