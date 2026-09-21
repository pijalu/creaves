package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

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
