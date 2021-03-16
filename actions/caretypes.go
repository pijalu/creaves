package actions

import (

  "fmt"
  "net/http"
  "github.com/gobuffalo/buffalo"
  "github.com/gobuffalo/pop/v5"
  "github.com/gobuffalo/x/responder"
  "creaves/models"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Caretype)
// DB Table: Plural (caretypes)
// Resource: Plural (Caretypes)
// Path: Plural (/caretypes)
// View Template Folder: Plural (/templates/caretypes/)

// CaretypesResource is the resource for the Caretype model
type CaretypesResource struct{
  buffalo.Resource
}

// List gets all Caretypes. This function is mapped to the path
// GET /caretypes
func (v CaretypesResource) List(c buffalo.Context) error {
  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  caretypes := &models.Caretypes{}

  // Paginate results. Params "page" and "per_page" control pagination.
  // Default values are "page=1" and "per_page=20".
  q := tx.PaginateFromParams(c.Params())

  // Retrieve all Caretypes from the DB
  if err := q.All(caretypes); err != nil {
    return err
  }

  return responder.Wants("html", func (c buffalo.Context) error {
    // Add the paginator to the context so it can be used in the template.
    c.Set("pagination", q.Paginator)

    c.Set("caretypes", caretypes)
    return c.Render(http.StatusOK, r.HTML("/caretypes/index.plush.html"))
  }).Wants("json", func (c buffalo.Context) error {
    return c.Render(200, r.JSON(caretypes))
  }).Wants("xml", func (c buffalo.Context) error {
    return c.Render(200, r.XML(caretypes))
  }).Respond(c)
}

// Show gets the data for one Caretype. This function is mapped to
// the path GET /caretypes/{caretype_id}
func (v CaretypesResource) Show(c buffalo.Context) error {
  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  // Allocate an empty Caretype
  caretype := &models.Caretype{}

  // To find the Caretype the parameter caretype_id is used.
  if err := tx.Find(caretype, c.Param("caretype_id")); err != nil {
    return c.Error(http.StatusNotFound, err)
  }

  return responder.Wants("html", func (c buffalo.Context) error {
    c.Set("caretype", caretype)

    return c.Render(http.StatusOK, r.HTML("/caretypes/show.plush.html"))
  }).Wants("json", func (c buffalo.Context) error {
    return c.Render(200, r.JSON(caretype))
  }).Wants("xml", func (c buffalo.Context) error {
    return c.Render(200, r.XML(caretype))
  }).Respond(c)
}

// New renders the form for creating a new Caretype.
// This function is mapped to the path GET /caretypes/new
func (v CaretypesResource) New(c buffalo.Context) error {
  c.Set("caretype", &models.Caretype{})

  return c.Render(http.StatusOK, r.HTML("/caretypes/new.plush.html"))
}
// Create adds a Caretype to the DB. This function is mapped to the
// path POST /caretypes
func (v CaretypesResource) Create(c buffalo.Context) error {
  // Allocate an empty Caretype
  caretype := &models.Caretype{}

  // Bind caretype to the html form elements
  if err := c.Bind(caretype); err != nil {
    return err
  }

  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  // Validate the data from the html form
  verrs, err := tx.ValidateAndCreate(caretype)
  if err != nil {
    return err
  }

  if verrs.HasAny() {
    return responder.Wants("html", func (c buffalo.Context) error {
      // Make the errors available inside the html template
      c.Set("errors", verrs)

      // Render again the new.html template that the user can
      // correct the input.
      c.Set("caretype", caretype)

      return c.Render(http.StatusUnprocessableEntity, r.HTML("/caretypes/new.plush.html"))
    }).Wants("json", func (c buffalo.Context) error {
      return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
    }).Wants("xml", func (c buffalo.Context) error {
      return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
    }).Respond(c)
  }

  return responder.Wants("html", func (c buffalo.Context) error {
    // If there are no errors set a success message
    c.Flash().Add("success", T.Translate(c, "caretype.created.success"))

    // and redirect to the show page
    return c.Redirect(http.StatusSeeOther, "/caretypes/%v", caretype.ID)
  }).Wants("json", func (c buffalo.Context) error {
    return c.Render(http.StatusCreated, r.JSON(caretype))
  }).Wants("xml", func (c buffalo.Context) error {
    return c.Render(http.StatusCreated, r.XML(caretype))
  }).Respond(c)
}

// Edit renders a edit form for a Caretype. This function is
// mapped to the path GET /caretypes/{caretype_id}/edit
func (v CaretypesResource) Edit(c buffalo.Context) error {
  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  // Allocate an empty Caretype
  caretype := &models.Caretype{}

  if err := tx.Find(caretype, c.Param("caretype_id")); err != nil {
    return c.Error(http.StatusNotFound, err)
  }

  c.Set("caretype", caretype)
  return c.Render(http.StatusOK, r.HTML("/caretypes/edit.plush.html"))
}
// Update changes a Caretype in the DB. This function is mapped to
// the path PUT /caretypes/{caretype_id}
func (v CaretypesResource) Update(c buffalo.Context) error {
  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  // Allocate an empty Caretype
  caretype := &models.Caretype{}

  if err := tx.Find(caretype, c.Param("caretype_id")); err != nil {
    return c.Error(http.StatusNotFound, err)
  }

  // Bind Caretype to the html form elements
  if err := c.Bind(caretype); err != nil {
    return err
  }

  verrs, err := tx.ValidateAndUpdate(caretype)
  if err != nil {
    return err
  }

  if verrs.HasAny() {
    return responder.Wants("html", func (c buffalo.Context) error {
      // Make the errors available inside the html template
      c.Set("errors", verrs)

      // Render again the edit.html template that the user can
      // correct the input.
      c.Set("caretype", caretype)

      return c.Render(http.StatusUnprocessableEntity, r.HTML("/caretypes/edit.plush.html"))
    }).Wants("json", func (c buffalo.Context) error {
      return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
    }).Wants("xml", func (c buffalo.Context) error {
      return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
    }).Respond(c)
  }

  return responder.Wants("html", func (c buffalo.Context) error {
    // If there are no errors set a success message
    c.Flash().Add("success", T.Translate(c, "caretype.updated.success"))

    // and redirect to the show page
    return c.Redirect(http.StatusSeeOther, "/caretypes/%v", caretype.ID)
  }).Wants("json", func (c buffalo.Context) error {
    return c.Render(http.StatusOK, r.JSON(caretype))
  }).Wants("xml", func (c buffalo.Context) error {
    return c.Render(http.StatusOK, r.XML(caretype))
  }).Respond(c)
}

// Destroy deletes a Caretype from the DB. This function is mapped
// to the path DELETE /caretypes/{caretype_id}
func (v CaretypesResource) Destroy(c buffalo.Context) error {
  // Get the DB connection from the context
  tx, ok := c.Value("tx").(*pop.Connection)
  if !ok {
    return fmt.Errorf("no transaction found")
  }

  // Allocate an empty Caretype
  caretype := &models.Caretype{}

  // To find the Caretype the parameter caretype_id is used.
  if err := tx.Find(caretype, c.Param("caretype_id")); err != nil {
    return c.Error(http.StatusNotFound, err)
  }

  if err := tx.Destroy(caretype); err != nil {
    return err
  }

  return responder.Wants("html", func (c buffalo.Context) error {
    // If there are no errors set a flash message
    c.Flash().Add("success", T.Translate(c, "caretype.destroyed.success"))

    // Redirect to the index page
    return c.Redirect(http.StatusSeeOther, "/caretypes")
  }).Wants("json", func (c buffalo.Context) error {
    return c.Render(http.StatusOK, r.JSON(caretype))
  }).Wants("xml", func (c buffalo.Context) error {
    return c.Render(http.StatusOK, r.XML(caretype))
  }).Respond(c)
}