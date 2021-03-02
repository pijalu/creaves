package actions

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/tags/form"
)

type selType struct {
	label string
	value interface{}
}

//SelectValue returns select value in a select
func (st *selType) SelectValue() interface{} {
	return st.value
}

//SelectLabel returns label to use in a select
func (st *selType) SelectLabel() string {
	return st.label
}

func animalTypes(c buffalo.Context) (*models.Animaltypes, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Animaltypes{}
	if err := tx.Order("name asc").All(ts); err != nil {
		return nil, err
	}
	c.Logger().Debugf("Loaded animal types: %v", ts)

	return ts, nil
}

func animalTypesToSelectables(ts *models.Animaltypes) form.Selectables {
	res := []form.Selectable{}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Name,
			value: ts.ID,
		})
	}
	return res
}

func outtakeTypes(c buffalo.Context) (*models.Outtaketypes, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Outtaketypes{}
	if err := tx.Order("name asc").All(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func outtakeTypesToSelectables(ts *models.Outtaketypes) form.Selectables {
	res := []form.Selectable{}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Name,
			value: ts.ID,
		})
	}
	return res
}

func animalages(c buffalo.Context) (*models.Animalages, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Animalages{}
	if err := tx.Order("name asc").All(ts); err != nil {
		return nil, err
	}
	c.Logger().Debugf("Loaded animal types: %v", ts)

	return ts, nil
}

func animalagesToSelectables(ts *models.Animalages) form.Selectables {
	res := []form.Selectable{}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Name,
			value: ts.ID,
		})
	}
	return res
}
