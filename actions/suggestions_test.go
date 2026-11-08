package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

// TestSuggestionsDrugRemark drives the full App() to verify the drug remark
// endpoint used by the treatment form to prefill remarks (issue #73).
func TestSuggestionsDrugRemark(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	tx := searchTestDB(t)

	drugName := "TS-DrugRem-" + uuid.Must(uuid.NewV4()).String()[:8]
	drug := &models.Drug{
		ID:          uuid.Must(uuid.NewV4()),
		Name:        drugName,
		Description: nulls.NewString("Anti-infectieux — 0.5ml par 500g"),
	}
	if err := tx.Create(drug); err != nil {
		t.Fatalf("drug fixture creation failed: %v", err)
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)

	// existing drug → its description
	resp, err := client.Get(baseURL + "/suggestions/drug_remark?name=" + url.QueryEscape(drugName))
	if err != nil {
		t.Fatalf("GET /suggestions/drug_remark: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", body, err)
	}
	if got["description"] != drug.Description.String {
		t.Fatalf("description = %q, want %q", got["description"], drug.Description.String)
	}

	// unknown drug → empty description (client no-ops)
	resp2, err := client.Get(baseURL + "/suggestions/drug_remark?name=TS-NoSuchDrug-" + drugName)
	if err != nil {
		t.Fatalf("GET unknown drug: %v", err)
	}
	defer resp2.Body.Close()
	var got2 map[string]string
	body2, _ := io.ReadAll(resp2.Body)
	if err := json.Unmarshal(body2, &got2); err != nil {
		t.Fatalf("invalid JSON %q: %v", body2, err)
	}
	if got2["description"] != "" {
		t.Fatalf("unknown drug description = %q, want empty", got2["description"])
	}
}

// TestSuggestionsOuttakePreciseLocation (#208.8): the endpoint returns
// distinct previously used precise-location values, filtered by q.
func TestSuggestionsOuttakePreciseLocation(t *testing.T) {
	tx := searchTestDB(t)

	marker := "TSPL-" + uuid.Must(uuid.NewV4()).String()[:8]
	ot := &models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "TSPL-Type-" + marker}
	if err := tx.Create(ot); err != nil {
		t.Fatalf("outtaketype fixture creation failed: %v", err)
	}
	o1 := &models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), TypeID: ot.ID, PreciseLocation: nulls.NewString(marker + " rue du Bois 12")}
	o2 := &models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), TypeID: ot.ID, PreciseLocation: nulls.NewString(marker + " avenue des Garennes")}
	o3 := &models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), TypeID: ot.ID, PreciseLocation: nulls.NewString("unrelated-" + marker)}
	for _, o := range []*models.Outtake{o1, o2, o3} {
		if err := tx.Create(o); err != nil {
			t.Fatalf("outtake fixture creation failed: %v", err)
		}
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM outtakes WHERE id IN (?, ?, ?)", o1.ID, o2.ID, o3.ID).Exec()
		models.DB.RawQuery("DELETE FROM outtaketypes WHERE id = ?", ot.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)

	resp, err := client.Get(baseURL + "/suggestions/outtake_precise_location?q=" + url.QueryEscape(marker))
	if err != nil {
		t.Fatalf("GET /suggestions/outtake_precise_location: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var got []string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", body, err)
	}
	if len(got) != 3 {
		t.Fatalf("got %v, want 3 values containing the marker", got)
	}

	// substring filter narrows the result
	resp2, err := client.Get(baseURL + "/suggestions/outtake_precise_location?q=" + url.QueryEscape(marker+" rue"))
	if err != nil {
		t.Fatalf("GET filtered: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	var got2 []string
	if err := json.Unmarshal(body2, &got2); err != nil {
		t.Fatalf("invalid JSON %q: %v", body2, err)
	}
	if len(got2) != 1 || got2[0] != marker+" rue du Bois 12" {
		t.Fatalf("filtered got %v", got2)
	}
}

// TestReceptionNewEntryCauseIndications (#208.9): the reception form
// embeds the entry-cause indications map so the select2 search matches the
// indication text as well as the label.
func TestReceptionNewEntryCauseIndications(t *testing.T) {
	tx := searchTestDB(t)

	marker := "TSIND-" + uuid.Must(uuid.NewV4()).String()[:8]
	ec := &models.EntryCause{
		ID:         "TSI-" + uuid.Must(uuid.NewV4()).String()[:8],
		Cause:      "Cause " + marker,
		Detail:     "Detail " + marker,
		Nature:     "Nature " + marker,
		Indication: "Indication " + marker,
	}
	if err := tx.Create(ec); err != nil {
		t.Fatalf("entry cause fixture creation failed: %v", err)
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM entry_causes WHERE id = ?", ec.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)
	resp, err := client.Get(baseURL + "/reception/new")
	if err != nil {
		t.Fatalf("GET /reception/new: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, `id="entry-cause-indications"`) {
		t.Fatalf("reception form lacks the entry-cause-indications JSON block")
	}
	if !strings.Contains(html, "Indication "+marker) {
		t.Fatalf("indications map missing fixture indication %q", "Indication "+marker)
	}
	if !strings.Contains(html, `ecIndications[data.id]`) {
		t.Fatalf("select2 matcher must consult the indications map")
	}
}

// TestSuggestionsAnimaltypeDefault verifies the reception wizard pre-fill
// source (issue #199-1): the endpoint returns the type's default species as a
// one-element array, and an empty array for a type without default — so the
// species field stays empty instead of copying the first suggestion.
func TestSuggestionsAnimaltypeDefault(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	tx := searchTestDB(t)

	marker := uuid.Must(uuid.NewV4()).String()[:8]
	withDefault := &models.Animaltype{
		ID:             uuid.Must(uuid.NewV4()),
		Name:           "TS-DefType-" + marker,
		DefaultSpecies: nulls.NewString("TS-DefSpecies-" + marker),
	}
	withoutDefault := &models.Animaltype{
		ID:   uuid.Must(uuid.NewV4()),
		Name: "TS-NoDefType-" + marker,
	}
	if err := tx.Create(withDefault); err != nil {
		t.Fatalf("create type with default: %v", err)
	}
	if err := tx.Create(withoutDefault); err != nil {
		t.Fatalf("create type without default: %v", err)
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM animaltypes WHERE id IN (?, ?)", withDefault.ID, withoutDefault.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)
	get := func(id string) []string {
		t.Helper()
		resp, err := client.Get(baseURL + "/suggestions/animaltype_default_species?animaltype_id=" + url.QueryEscape(id))
		if err != nil {
			t.Fatalf("GET /suggestions/animaltype_default_species: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var got []string
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("invalid JSON %q: %v", body, err)
		}
		return got
	}

	if got := get(withDefault.ID.String()); len(got) != 1 || got[0] != withDefault.DefaultSpecies.String {
		t.Fatalf("type with default → %v, want [%q]", got, withDefault.DefaultSpecies.String)
	}
	if got := get(withoutDefault.ID.String()); len(got) != 0 {
		t.Fatalf("type without default → %v, want empty", got)
	}
	if got := get(uuid.Must(uuid.NewV4()).String()); len(got) != 0 {
		t.Fatalf("unknown type → %v, want empty", got)
	}
	if got := get(""); len(got) != 0 {
		t.Fatalf("missing type param → %v, want empty", got)
	}
}
