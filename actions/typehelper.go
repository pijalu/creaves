package actions

import (
	"creaves/models"
	"fmt"
	"regexp"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/tags/form"
	"github.com/gofrs/uuid"
)

type selType struct {
	label string
	value interface{}
}

// SelectValue returns select value in a select
func (st *selType) SelectValue() interface{} {
	return st.value
}

// SelectLabel returns label to use in a select
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
	//c.Logger().Debugf("Loaded animal types: %v", ts)

	return ts, nil
}

func animalTypesToSelectables(ts *models.Animaltypes) form.Selectables {
	res := []form.Selectable{}
	removeEmpty := false

	res = append(res, &selType{label: "", value: ""})

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Name,
			value: ts.ID,
		})
		removeEmpty = removeEmpty || ts.Default
	}

	if removeEmpty {
		return res[1:]
	}

	return res
}

func defZone(c buffalo.Context) (*models.Zone, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Zone{}
	if err := tx.Where("`default` is true").Order("zone asc").First(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func zones(c buffalo.Context) (*models.Zones, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.Zones{}
	if err := tx.Order("zone asc").All(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func zonesMap(c buffalo.Context) (map[string]string, error) {
	zs, err := zones(c)
	if err != nil {
		return nil, err
	}

	ret := map[string]string{}
	for _, z := range *zs {
		ret[z.Zone] = z.Type
	}

	return ret, nil
}

func zonesToSelectables(ts *models.Zones) form.Selectables {
	res := []form.Selectable{}
	//removeEmpty := false

	res = append(res, &selType{label: "", value: ""})

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Zone,
			value: ts.Zone,
		})
		//removeEmpty = removeEmpty || ts.Default
	}
	/*
		if removeEmpty {
			return res[1:]
		}
	*/

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
	//c.Logger().Debugf("Loaded animal types: %v", ts)

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
	//c.Logger().Debugf("Loaded users: %v", u)

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

func selectFeedingPeriod() form.Selectables {
	res := []form.Selectable{}

	// custom minutes
	res = append(res,
		&selType{
			label: "N.A.",
			value: 0,
		},
		&selType{
			label: "15min",
			value: 15,
		},
		&selType{
			label: "30min",
			value: 30,
		},
	)

	// Up to 12h
	for h := 1; h <= 12; h++ {
		res = append(res, &selType{
			label: fmt.Sprintf("%dh", h),
			value: h * 60,
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

// AnimalYearNumberRegEx
var AnimalYearNumberRegEx = regexp.MustCompile(`(\d+)(/(\d{2}))?`)

func entryCauses(c buffalo.Context) (*models.EntryCauses, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ts := &models.EntryCauses{}
	if err := tx.Order("sort_order asc").All(ts); err != nil {
		return nil, err
	}

	return ts, nil
}

func entryCausesToSelectables(ts *models.EntryCauses, withBlank bool) form.Selectables {
	res := []form.Selectable{}
	if withBlank {
		res = append(res, &selType{label: " ", value: ""})
	}

	for _, ts := range *ts {
		res = append(res, &selType{
			label: ts.Fmt(true),
			value: ts.ID,
		})
	}
	return res
}
