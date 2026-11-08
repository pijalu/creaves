package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"

	"creaves/models"
)

// Care templates are a shared, admin-managed pool of reusable note texts for
// "suivi" cares (introduced per-user in issue #158, moved to an admin CRUD by
// bugs.md bug 6). The care form only consumes the pool through
// GET /suggestions/care_templates; management happens in the pages below.

// careTemplateView is a CareTemplate enriched with the owner's login and the
// default care type name for the admin list page.
type careTemplateView struct {
	models.CareTemplate
	Owner        string `json:"owner"`
	CaretypeName string `json:"caretype_name"`
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
	ctNames := map[uuid.UUID]string{}
	for i, t := range *templates {
		views[i] = careTemplateView{CareTemplate: t}
		if _, seen := owners[t.UserID]; !seen {
			u := &models.User{}
			if err := tx.Find(u, t.UserID); err == nil {
				owners[t.UserID] = u.Login
			}
		}
		views[i].Owner = owners[t.UserID]
		if t.CaretypeID.Valid {
			if _, seen := ctNames[t.CaretypeID.UUID]; !seen {
				ctn := &models.Caretype{}
				if err := tx.Find(ctn, t.CaretypeID.UUID); err == nil {
					ctNames[t.CaretypeID.UUID] = ctn.Name
				}
			}
			views[i].CaretypeName = ctNames[t.CaretypeID.UUID]
		}
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
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	sel, err := careTemplateCaretypes(c, tx)
	if err != nil {
		return err
	}
	c.Set("selectCaretype", sel)
	return c.Render(http.StatusOK, r.HTML("care_templates/new.plush.html"))
}

// careTypeOption is a template-friendly (exported fields) care type option
// for the care template admin form select (#208.17).
type careTypeOption struct {
	Value string
	Label string
}

// careTemplateCaretypes loads the care types for the admin template form
// select (#208.17).
func careTemplateCaretypes(c buffalo.Context, tx *pop.Connection) ([]careTypeOption, error) {
	ct := &models.Caretypes{}
	if err := tx.Order("name asc").All(ct); err != nil {
		return nil, err
	}
	sel := caretypesToSelectables(ct, currentLang(c), tx)
	out := make([]careTypeOption, 0, len(sel))
	for _, s := range sel {
		v := ""
		if sv, ok := s.SelectValue().(uuid.UUID); ok {
			v = sv.String()
		}
		out = append(out, careTypeOption{Value: v, Label: s.SelectLabel()})
	}
	return out, nil
}

// bindCareTemplateDefault reads the non-model form fields of the care
// template form (#208.17): the care type select (named "DefaultCaretypeID"
// so c.Bind does not choke on an empty value) and the IsDefault checkbox.
func bindCareTemplateDefault(c buffalo.Context, tpl *models.CareTemplate) error {
	tpl.IsDefault = c.Param("IsDefault") == "true" || c.Param("IsDefault") == "on"
	ctID := c.Param("DefaultCaretypeID")
	if ctID == "" {
		tpl.CaretypeID = nulls.UUID{}
		return nil
	}
	id, err := uuid.FromString(ctID)
	if err != nil {
		tpl.CaretypeID = nulls.UUID{}
		return nil
	}
	tpl.CaretypeID = nulls.NewUUID(id)
	return nil
}

// clearOtherDefaults enforces a single default template per care type
// (#208.17): after saving tpl as the default of its care type, every other
// template of the same care type loses the flag.
func clearOtherDefaults(tx *pop.Connection, tpl *models.CareTemplate) error {
	if !tpl.IsDefault || !tpl.CaretypeID.Valid {
		return nil
	}
	return tx.RawQuery("UPDATE care_templates SET is_default = 0 WHERE caretype_id = ? AND id != ?", tpl.CaretypeID.UUID.String(), tpl.ID.String()).Exec()
}

// localizeCareTemplateErrors replaces the stock English validator messages
// ("Name can not be blank.") with localized ones: the form helpers render the
// verrs strings verbatim, so translation must happen handler-side (#205-12).
func localizeCareTemplateErrors(c buffalo.Context, verrs *validate.Errors) *validate.Errors {
	out := validate.NewErrors()
	for field, msgs := range verrs.Errors {
		for _, m := range msgs {
			switch m {
			case "Name can not be blank.":
				out.Add(field, T.Translate(c, "care_templates.errors.name_blank"))
			case "Content can not be blank.":
				out.Add(field, T.Translate(c, "care_templates.errors.content_blank"))
			default:
				out.Add(field, m)
			}
		}
	}
	return out
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
	if err := bindCareTemplateDefault(c, tpl); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndCreate(tpl)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", localizeCareTemplateErrors(c, verrs))
		c.Set("tpl", tpl)
		sel, serr := careTemplateCaretypes(c, tx)
		if serr != nil {
			return serr
		}
		c.Set("selectCaretype", sel)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("care_templates/new.plush.html"))
	}
	if err := clearOtherDefaults(tx, tpl); err != nil {
		return err
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
	sel, err := careTemplateCaretypes(c, tx)
	if err != nil {
		return err
	}
	c.Set("selectCaretype", sel)
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
	if err := bindCareTemplateDefault(c, tpl); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(tpl)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", localizeCareTemplateErrors(c, verrs))
		c.Set("tpl", tpl)
		sel, serr := careTemplateCaretypes(c, tx)
		if serr != nil {
			return serr
		}
		c.Set("selectCaretype", sel)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("care_templates/edit.plush.html"))
	}
	if err := clearOtherDefaults(tx, tpl); err != nil {
		return err
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
// With ?default_for=<caretype_id> it returns only the default template of
// that care type (#208.17) — an empty array when none exists.
func SuggestionsCareTemplates(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	if GetCurrentUser(c) == nil {
		return c.Error(http.StatusUnauthorized, fmt.Errorf("not authenticated"))
	}
	if ct := c.Param("default_for"); ct != "" {
		templates := &models.CareTemplates{}
		if err := tx.Where("caretype_id = ? AND is_default = 1", ct).All(templates); err != nil {
			return err
		}
		return c.Render(http.StatusOK, r.JSON(templates))
	}
	templates := &models.CareTemplates{}
	if err := tx.Order("name asc").All(templates); err != nil {
		return err
	}
	return c.Render(http.StatusOK, r.JSON(templates))
}
