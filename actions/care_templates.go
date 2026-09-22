package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"

	"creaves/models"
)

// Care templates are a shared, admin-managed pool of reusable note texts for
// "suivi" cares (introduced per-user in issue #158, moved to an admin CRUD by
// bugs.md bug 6). The care form only consumes the pool through
// GET /suggestions/care_templates; management happens in the pages below.

// careTemplateView is a CareTemplate enriched with the owner's login for the
// admin list page.
type careTemplateView struct {
	models.CareTemplate
	Owner string `json:"owner"`
}

// careTemplatesAdminDenied reports whether the current user may not manage
// note templates: unauthenticated → 401; non-admin → warning flash and the
// caller must return the home redirect itself (c.Redirect writes the response
// and returns nil, so it only terminates as the handler's return value).
func careTemplatesAdminDenied(c buffalo.Context) bool {
	user := GetCurrentUser(c)
	if user == nil {
		_ = c.Error(http.StatusUnauthorized, fmt.Errorf("not authenticated"))
		return true
	}
	if !user.Admin {
		c.Flash().Add("warning", T.Translate(c, "care_templates.admin.restricted"))
		return true
	}
	return false
}

// CareTemplatesResource is the admin CRUD resource for CareTemplate.
type CareTemplatesResource struct {
	buffalo.Resource
}

// List gets all note templates. GET /care_templates
func (v CareTemplatesResource) List(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	templates := &models.CareTemplates{}
	q := tx.PaginateFromParams(c.Params())
	if err := q.Order("name asc").All(templates); err != nil {
		return err
	}

	views := make([]careTemplateView, len(*templates))
	owners := map[uuid.UUID]string{}
	for i, t := range *templates {
		views[i] = careTemplateView{CareTemplate: t}
		if _, seen := owners[t.UserID]; !seen {
			u := &models.User{}
			if err := tx.Find(u, t.UserID); err == nil {
				owners[t.UserID] = u.Login
			}
		}
		views[i].Owner = owners[t.UserID]
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("pagination", q.Paginator)
		c.Set("templates", views)
		return c.Render(http.StatusOK, r.HTML("care_templates/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(views))
	}).Respond(c)
}

// Show is not part of the admin CRUD; it redirects to the list.
// GET /care_templates/{care_template_id}
func (v CareTemplatesResource) Show(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	return c.Redirect(http.StatusSeeOther, "/care_templates")
}

// New renders the creation form. GET /care_templates/new
func (v CareTemplatesResource) New(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	c.Set("tpl", &models.CareTemplate{})
	return c.Render(http.StatusOK, r.HTML("care_templates/new.plush.html"))
}

// Create stores a new note template owned by the current admin.
// POST /care_templates
func (v CareTemplatesResource) Create(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	tpl := &models.CareTemplate{}
	if err := c.Bind(tpl); err != nil {
		return err
	}
	tpl.ID = uuid.Must(uuid.NewV4())
	tpl.UserID = GetCurrentUser(c).ID

	verrs, err := tx.ValidateAndCreate(tpl)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", verrs)
		c.Set("tpl", tpl)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("care_templates/new.plush.html"))
	}

	c.Flash().Add("success", T.Translate(c, "care_templates.created.success"))
	return c.Redirect(http.StatusSeeOther, "/care_templates")
}

// Edit renders the update form. GET /care_templates/{care_template_id}/edit
func (v CareTemplatesResource) Edit(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	tpl := &models.CareTemplate{}
	if err := tx.Find(tpl, c.Param("care_template_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("tpl", tpl)
	return c.Render(http.StatusOK, r.HTML("care_templates/edit.plush.html"))
}

// Update changes a note template. PUT /care_templates/{care_template_id}
func (v CareTemplatesResource) Update(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	tpl := &models.CareTemplate{}
	if err := tx.Find(tpl, c.Param("care_template_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := c.Bind(tpl); err != nil {
		return err
	}
	// UserID (the creator) is intentionally left untouched on update.

	verrs, err := tx.ValidateAndUpdate(tpl)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", verrs)
		c.Set("tpl", tpl)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("care_templates/edit.plush.html"))
	}

	c.Flash().Add("success", T.Translate(c, "care_templates.updated.success"))
	return c.Redirect(http.StatusSeeOther, "/care_templates")
}

// Destroy deletes a note template. DELETE /care_templates/{care_template_id}
func (v CareTemplatesResource) Destroy(c buffalo.Context) error {
	if careTemplatesAdminDenied(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	tpl := &models.CareTemplate{}
	if err := tx.Find(tpl, c.Param("care_template_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := tx.Destroy(tpl); err != nil {
		return err
	}

	c.Flash().Add("success", T.Translate(c, "care_templates.destroyed.success"))
	return c.Redirect(http.StatusSeeOther, "/care_templates")
}

// SuggestionsCareTemplates serves the shared template pool (all users'
// templates) to any authenticated user, for the care form select.
// GET /suggestions/care_templates
func SuggestionsCareTemplates(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	if GetCurrentUser(c) == nil {
		return c.Error(http.StatusUnauthorized, fmt.Errorf("not authenticated"))
	}
	templates := &models.CareTemplates{}
	if err := tx.Order("name asc").All(templates); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.JSON(templates))
}
