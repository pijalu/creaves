package actions

import (
	"os"
	"testing"
)

func TestDumpPhase0bFresh(t *testing.T) {
	if os.Getenv("PHASE0B_DUMP") != "1" {
		t.Skip("dump only")
	}
	_, client, baseURL := planFixtureRich(t)
	for _, kind := range phase0bKinds {
		raw := fetchPlanHTML(t, client, baseURL, "?kind="+kind)
		norm := normalizePlanDOM(t, raw)
		os.WriteFile("/tmp/fresh_"+kind+".txt", []byte(norm), 0644)
	}
}
