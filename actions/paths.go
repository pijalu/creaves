package actions

import (
	"net/http"

	"github.com/gobuffalo/buffalo"
)

// PathHandler is a default handler to serve up
// a home page.
func PathHandler(c buffalo.Context) error {
	return c.Render(http.StatusOK, r.HTML("paths/index.html"))
}
