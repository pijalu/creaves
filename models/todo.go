package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// TODO status colors for the dashboard block (issue #147):
// done or future → green; open, todo date within the last 8 hours →
// yellow; open, todo date older than 8 hours → red.
const (
	TodoStatusGreen  = "green"
	TodoStatusYellow = "yellow"
	TodoStatusRed    = "red"

	// TodoOverdueAfter is how long an open todo may pass its todo_date
	// before it turns red.
	TodoOverdueAfter = 8 * time.Hour
)

// Todo is used by pop to map your todos database table to your go code.
// TableName pins the table name: pop would otherwise pluralize to
// "todoes" (issue #147 uses "todos").
type Todo struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Description string     `json:"description" db:"description"`
	TodoDate    time.Time  `json:"todo_date" db:"todo_date"`
	DoneAt      nulls.Time `json:"done_at" db:"done_at"`
	DoneByID    nulls.UUID `json:"done_by_id" db:"done_by_id"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// TableName sets the table name for the Todo model.
func (Todo) TableName() string {
	return "todos"
}

// Todos is not required by pop and may be removed.
type Todos []Todo

// String is not required by pop and may be removed.
func (t Todo) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// String is not required by pop and may be removed.
func (t Todos) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (t *Todo) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: t.Description, Name: "Description"},
	), nil
}

// IsDone reports whether the todo has been confirmed done.
func (t Todo) IsDone() bool {
	return t.DoneAt.Valid
}

// Status classifies the todo for the dashboard color code (issue #147):
// done or future → green; open within 8h of todo_date → yellow;
// open older than 8h → red.
func (t Todo) Status(now time.Time) string {
	if t.IsDone() {
		return TodoStatusGreen
	}
	if t.TodoDate.After(now) {
		return TodoStatusGreen
	}
	if now.Sub(t.TodoDate) <= TodoOverdueAfter {
		return TodoStatusYellow
	}
	return TodoStatusRed
}
