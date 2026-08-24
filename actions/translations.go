package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

type TranslationsResource struct{ buffalo.Resource }

type translationRow struct {
	models.Translation
	BaseValue string `db:"base_value"`
}

type translationRows []translationRow

func (t translationRows) String() string { return fmt.Sprintf("%v", []translationRow(t)) }

func translationAdmin(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	return nil
}

func (v TranslationsResource) List(c buffalo.Context) error {
	if err := translationAdmin(c); err != nil {
		return err
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	search := strings.TrimSpace(c.Param("q"))
	rows := translationRows{}
	query := tx.RawQuery("SELECT tr.id, tr.table_name, tr.record_id, tr.field, tr.locale, tr.value, tr.created_at, tr.updated_at, COALESCE(fr.value, '') AS base_value FROM translations tr LEFT JOIN translations fr ON fr.table_name=tr.table_name AND fr.record_id=tr.record_id AND fr.field=tr.field AND fr.locale='fr' WHERE (? = '' OR tr.value LIKE ? OR fr.value LIKE ?) ORDER BY tr.table_name, tr.field, tr.locale, tr.record_id", search, "%"+search+"%", "%"+search+"%")
	if err := query.All(&rows); err != nil {
		return err
	}
	c.Set("translations", rows)
	c.Set("translationSearch", search)
	return c.Render(http.StatusOK, r.HTML("/translations/index.plush.html"))
}

func (v TranslationsResource) Show(c buffalo.Context) error {
	if err := translationAdmin(c); err != nil {
		return err
	}
	tx := c.Value("tx").(*pop.Connection)
	row := &translationRow{}
	if err := tx.RawQuery("SELECT tr.id, tr.table_name, tr.record_id, tr.field, tr.locale, tr.value, tr.created_at, tr.updated_at, COALESCE(fr.value, '') AS base_value FROM translations tr LEFT JOIN translations fr ON fr.table_name=tr.table_name AND fr.record_id=tr.record_id AND fr.field=tr.field AND fr.locale='fr' WHERE tr.id = ?", c.Param("translation_id")).First(row); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	c.Set("translation", row)
	return c.Render(http.StatusOK, r.HTML("/translations/show.plush.html"))
}

func (v TranslationsResource) Edit(c buffalo.Context) error {
	if err := translationAdmin(c); err != nil {
		return err
	}
	tx := c.Value("tx").(*pop.Connection)
	translation := &models.Translation{}
	if err := tx.Find(translation, c.Param("translation_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	c.Set("translation", translation)
	return c.Render(http.StatusOK, r.HTML("/translations/edit.plush.html"))
}

func (v TranslationsResource) Update(c buffalo.Context) error {
	if err := translationAdmin(c); err != nil {
		return err
	}
	tx := c.Value("tx").(*pop.Connection)
	translation := &models.Translation{}
	if err := tx.Find(translation, c.Param("translation_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := c.Bind(translation); err != nil {
		return err
	}
	if strings.TrimSpace(translation.Value) == "" {
		c.Set("translation", translation)
		c.Flash().Add("danger", "Value cannot be empty")
		return c.Render(http.StatusUnprocessableEntity, r.HTML("/translations/edit.plush.html"))
	}
	if err := tx.Update(translation); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/translations/%s", translation.ID)
}

func (v TranslationsResource) Destroy(c buffalo.Context) error {
	if err := translationAdmin(c); err != nil {
		return err
	}
	tx := c.Value("tx").(*pop.Connection)
	translation := &models.Translation{}
	if err := tx.Find(translation, c.Param("translation_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := tx.Destroy(translation); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/translations")
}

var _ = responder.Wants
