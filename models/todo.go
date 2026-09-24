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

	// TodoDueSoonWindow is the look-ahead window for the dashboard block
	// and the /todos main list (bugs.md TODO item): only open todos due
	// within the next 8 hours — or already overdue — are shown there;
	// todos due further out live in the collapsed "later" section.
	TodoDueSoonWindow = 8 * time.Hour
)

// Recurrence values for recurring todos (issue #199-8). Empty means a
// one-shot todo; the other values spawn the next occurrence when the
// todo is marked done.
const (
	TodoRecurrenceNone    = ""
	TodoRecurrenceWeekly  = "weekly"
	TodoRecurrenceMonthly = "monthly"
	TodoRecurrenceYearly  = "yearly"
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
	Recurrence  string     `json:"recurrence" db:"recurrence"`
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

// DueSoon reports whether an open todo is due within TodoDueSoonWindow of
// now — i.e. overdue already or due in the next 8 hours (bugs.md TODO
// item). Done todos are never "due soon".
func (t Todo) DueSoon(now time.Time) bool {
	if t.IsDone() {
		return false
	}
	return !t.TodoDate.After(now.Add(TodoDueSoonWindow))
}

// NextOccurrence returns the follow-up todo to create when a recurring
// todo is marked done (issue #199-8), or nil for one-shot todos.
// The next todo date is shifted by +1 week/month/year.
func (t Todo) NextOccurrence() *Todo {
	var next time.Time
	switch t.Recurrence {
	case TodoRecurrenceWeekly:
		next = t.TodoDate.AddDate(0, 0, 7)
	case TodoRecurrenceMonthly:
		next = t.TodoDate.AddDate(0, 1, 0)
	case TodoRecurrenceYearly:
		next = t.TodoDate.AddDate(1, 0, 0)
	default:
		return nil
	}
	return &Todo{
		Description: t.Description,
		TodoDate:    next,
		Recurrence:  t.Recurrence,
	}
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
