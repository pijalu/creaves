package actions

import (
	"creaves/public"
	"creaves/templates"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/gobuffalo/buffalo/render"
	"github.com/gobuffalo/plush/v4"
)

// uiLanguages lists all selectable UI languages (cookie value, native label).
var uiLanguages = []struct {
	code  string
	label string
}{
	{"fr", "Français"},
	{"en-US", "English"},
	{"de", "Deutsch"},
	{"nl", "Nederlands"},
}

// langLinks renders one link per UI language, except the current one.
// linkClass "nav-link" wraps each link in a <li class="nav-item"> for the
// anonymous navbar; anything else renders plain dropdown-item anchors.
func langLinks(target any, linkClass string, help plush.HelperContext) (template.HTML, error) {
	targetURL := fmt.Sprintf("%v", target)
	cur := ""
	if req, ok := help.Value("request").(*http.Request); ok {
		if cookie, err := req.Cookie("lang"); err == nil {
			cur = normalizeUILang(cookie.Value)
		}
	}
	var b strings.Builder
	for _, l := range uiLanguages {
		code := l.code
		norm := code
		if code == "fr" {
			norm = "" // base/canonical French
		}
		if norm == cur {
			continue
		}
		href := fmt.Sprintf("/lang/?lang=%s&url=%s", code, url.QueryEscape(targetURL))
		if linkClass == "nav-link" {
			fmt.Fprintf(&b, `<li class="nav-item"><a class="nav-link" href="%s">%s</a></li>`, href, l.label)
		} else {
			fmt.Fprintf(&b, `<a class="dropdown-item" href="%s">%s</a>`, href, l.label)
		}
	}
	return template.HTML(b.String()), nil
}

// normalizeUILang maps a lang cookie value to the comparison domain used by
// langLinks: "" for base/canonical French, otherwise the full code.
func normalizeUILang(lang string) string {
	switch lang {
	case "", "fr", "fr-FR":
		return ""
	case "en", "en-US":
		return "en-US"
	default:
		return lang
	}
}

var r *render.Engine

func init() {
	r = render.New(render.Options{
		// HTML layout to be used for all HTML requests:
		HTMLLayout: "application.plush.html",

		// Box containing all of the templates:
		TemplatesFS: templates.FS(),
		AssetsFS:    public.FS(),

		// Add template helpers here:
		Helpers: render.Helpers{
			"langLinks": langLinks,
			"bool2html": func(s bool) string {
				if s {
					return "✓"
				} else {
					return "×"
				}
			},
			"dbgDump": func(s any) string {
				return fmt.Sprintf("%v", s)
			},
		},
	})
}
