package actions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceDeleteTemplatesUseExplicitDeleteFlow(t *testing.T) {
	resources := []string{"animalages", "animaltypes", "caretypes", "discoverers", "drugs", "outtaketypes", "traveltypes"}
	for _, resource := range resources {
		for _, suffix := range []string{"", ".fr", ".de", ".nl"} {
			for _, page := range []string{"index", "show"} {
				path := filepath.Join("..", "templates", resource, page+".plush"+suffix+".html")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				s := string(data)
				if strings.Contains(s, `data-method": "DELETE"`) || strings.Contains(s, "/remap") || strings.Contains(s, "replacement_id") {
					t.Errorf("%s retains inline destructive/remap flow", path)
				}
				if !strings.Contains(s, "/"+resource+`/" + `) || !strings.Contains(s, "/delete") {
					t.Errorf("%s lacks explicit delete flow", path)
				}
			}
		}
	}
}
