package actions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gobuffalo/buffalo"
)

// TestParamIsTrueCheckedBoxes proves a checked box (hidden "false" + checkbox
// "true", as rendered by templates/config/_form.plush*.html) reads as true
// despite c.Param returning the first value ("false").
func TestParamIsTrueCheckedBoxes(t *testing.T) {
	for _, key := range []string{"Settings.EnableEventStream", "Settings.WebhookEnabled", "Active"} {
		vals := url.Values{}
		vals.Add(key, "false")
		vals.Add(key, "true")
		req := httptest.NewRequest(http.MethodPost, "/config/", strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if err := req.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := req.Form[key]; len(got) != 2 || got[0] != "false" {
			t.Fatalf("precondition: %q form values = %q, want [false true]", key, got)
		}

		a := buffalo.New(buffalo.Options{Env: "test"})
		var seen bool
		a.POST("/probe/", func(c buffalo.Context) error {
			seen = paramIsTrue(c, key)
			return c.Render(http.StatusOK, nil)
		})
		w := httptest.NewRecorder()
		probe := httptest.NewRequest(http.MethodPost, "/probe/", strings.NewReader(vals.Encode()))
		probe.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		a.ServeHTTP(w, probe)
		if !seen {
			t.Errorf("paramIsTrue(%q) with [false true] = false, want true", key)
		}
	}
}

// TestParamIsTrueUncheckedBox proves a lone hidden "false" still reads false.
func TestParamIsTrueUncheckedBox(t *testing.T) {
	a := buffalo.New(buffalo.Options{Env: "test"})
	var seen bool
	a.POST("/probe/", func(c buffalo.Context) error {
		seen = paramIsTrue(c, "Settings.WebhookEnabled")
		return c.Render(http.StatusOK, nil)
	})
	vals := url.Values{"Settings.WebhookEnabled": {"false"}}
	w := httptest.NewRecorder()
	probe := httptest.NewRequest(http.MethodPost, "/probe/", strings.NewReader(vals.Encode()))
	probe.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.ServeHTTP(w, probe)
	if seen {
		t.Error("paramIsTrue(unchecked) = true, want false")
	}
}
