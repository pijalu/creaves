package actions

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/tags/form"
	"github.com/gofrs/uuid"
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

func caretypes(c buffalo.Context) (*models.Caretypes, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Caretypes{}
	if err := tx.Order("name asc").All(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func caretypesToSelectables(ts *models.Caretypes) form.Selectables {
	res := []form.Selectable{}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Name,
			value: ts.ID,
		})
	}
	return res
}

func traveltypes(c buffalo.Context) (*models.Traveltypes, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Traveltypes{}
	if err := tx.Order("name asc").All(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func traveltypesToSelectables(ts *models.Traveltypes) form.Selectables {
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

func users(c buffalo.Context) (*models.Users, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	u := &models.Users{}
	if err := tx.Order("login asc").All(u); err != nil {
		return nil, err
	}
	c.Logger().Debugf("Loaded users: %v", u)

	return u, nil
}

func usersToMap(us *models.Users) map[uuid.UUID]models.User {
	m := make(map[uuid.UUID]models.User)

	for _, t := range *us {
		m[t.ID] = t
	}

	return m
}

func usersToSelectables(ts *models.Users) form.Selectables {
	res := []form.Selectable{}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Login,
			value: ts.ID,
		})
	}
	return res
}

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
