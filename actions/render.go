package actions

import (
	"creaves/public"
	"creaves/templates"
	"fmt"

	"github.com/gobuffalo/buffalo/render"
)

// R is the global render engine
var R *render.Engine

func init() {
	R = render.New(render.Options{
		// HTML layout to be used for all HTML requests:
		HTMLLayout: "application.plush.html",

		// Box containing all of the templates:
		TemplatesFS: templates.FS(),
		AssetsFS:    public.FS(),

		// Add template helpers here:
		Helpers: render.Helpers{
			"bool2html": func(s bool) string {
				if s {
					return "✓"
				} else {
					return "🞩"
				}
			},
			"dbgDump": func(s any) string {
				return fmt.Sprintf("%v", s)
			},
		},
	})

	// Also assign to old variable for backward compatibility
	r = R
}

// For backward compatibility
var r *render.Engine
