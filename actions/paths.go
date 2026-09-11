package actions

import (
	"net/http"

	"github.com/gobuffalo/buffalo"
)

// PathHandler serves route inventory to maintainers only.
// A home page.
func PathHandler(c buffalo.Context) error {
	if _, err := requireMaintainer(c); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.HTML("paths/index.html"))
}
