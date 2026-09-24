package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// TODO list (issue #147). Any user can create/edit/update todos and mark
// them done; only admins can delete or reopen.

// parseTodoDate parses the datetime-local form value with tolerant layouts,
// defaulting to now when empty or unparsable. Parsed in the UTC frame on
// purpose: the MySQL DSN has no loc parameter, so the driver stores the UTC
// wall clock into the naive DATETIME column (same convention as the buffalo
// binder, which parses custom layouts with time.Parse — see
// models.FormWallClockNow). Parsing in time.Local would shift the stored and
// displayed time by the local UTC offset (bugs.md calendar item).
func parseTodoDate(raw string) time.Time {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return models.FormWallClockNow()
}

// todoView carries the precomputed display fields for the templates:
// color badge class and the login of the user who marked it done.
type todoView struct {
	Todo   models.Todo
	Color  string
	DoneBy string
}

// badgeClass maps a todo status color to its Bootstrap badge class.
func badgeClass(status string) string {
	switch status {
	case models.TodoStatusGreen:
		return "success"
	case models.TodoStatusYellow:
		return "warning"
	default:
		return "danger"
	}
}

// todoViews converts todos into template views, resolving done-by logins.
func todoViews(tx *pop.Connection, todos models.Todos) ([]todoView, error) {
	logins := map[uuid.UUID]string{}
	ids := []uuid.UUID{}
	for _, t := range todos {
		if t.DoneByID.Valid {
			ids = append(ids, t.DoneByID.UUID)
		}
	}
	if len(ids) > 0 {
		us := []models.User{}
		if err := tx.Where("id in (?)", ids).All(&us); err != nil {
			return nil, err
		}
		for _, u := range us {
			logins[u.ID] = u.Login
		}
	}
	// TodoDate values live in the UTC wall-clock frame (naive DATETIME
	// column, DSN without loc): classify against the same frame.
	now := models.FormWallClockNow()
	views := make([]todoView, 0, len(todos))
	for _, t := range todos {
		views = append(views, todoView{
			Todo:   t,
			Color:  badgeClass(t.Status(now)),
			DoneBy: logins[t.DoneByID.UUID],
		})
	}
	return views, nil
}

// TodosIndex handles GET /todos: open todos due within TodoDueSoonWindow
// (or overdue) first (todo_date ascending), then a collapsed section with
// the open todos due further out, then the done section (most recently
// completed first). The due/later split is the bugs.md TODO item; both
// sections stay open in the DB — nothing is auto-marked done.
func TodosIndex(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	open := &models.Todos{}
	if err := tx.Where("done_at IS NULL").Order("todo_date asc").All(open); err != nil {
		return err
	}
	done := &models.Todos{}
	if err := tx.Where("done_at IS NOT NULL").Order("done_at desc").All(done); err != nil {
		return err
	}

	// Split open todos: due within the next 8h (or overdue) vs later.
	// todo_date lives in the UTC wall-clock frame (see listDashboardTodos).
	now := models.FormWallClockNow()
	due := models.Todos{}
	later := models.Todos{}
	for _, t := range *open {
		if t.DueSoon(now) {
			due = append(due, t)
		} else {
			later = append(later, t)
		}
	}

	openViews, err := todoViews(tx, due)
	if err != nil {
		return err
	}
	laterViews, err := todoViews(tx, later)
	if err != nil {
		return err
	}
	doneViews, err := todoViews(tx, *done)
	if err != nil {
		return err
	}

	c.Set("openTodos", openViews)
	c.Set("laterTodos", laterViews)
	c.Set("doneTodos", doneViews)
	c.Set("isAdmin", GetCurrentUser(c).Admin)

	return c.Render(http.StatusOK, r.HTML("todos/todos.plush.html"))
}

// TodosNew handles GET /todos/new: empty form, todo_date defaults to now.
func TodosNew(c buffalo.Context) error {
	todo := &models.Todo{TodoDate: models.FormWallClockNow()}
	c.Set("todo", todo)
	return c.Render(http.StatusOK, r.HTML("todos/new.plush.html"))
}

// TodosCreate handles POST /todos (any user).
func TodosCreate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	todo := &models.Todo{
		Description: c.Param("description"),
		TodoDate:    parseTodoDate(c.Param("todo_date")),
		Recurrence:  todoRecurrenceParam(c),
	}

	verrs, err := tx.ValidateAndCreate(todo)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", verrs)
		c.Set("todo", todo)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("todos/new.plush.html"))
	}

	c.Flash().Add("success", T.Translate(c, "todos.created.success"))
	return c.Redirect(http.StatusSeeOther, "/todos")
}

// loadTodo fetches a todo by id param or returns 404.
func loadTodo(c buffalo.Context, tx *pop.Connection) (*models.Todo, error) {
	todo := &models.Todo{}
	if err := tx.Find(todo, c.Param("todo_id")); err != nil {
		return nil, err
	}
	return todo, nil
}

// TodosEdit handles GET /todos/{todo_id}/edit (any user).
func TodosEdit(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	todo, err := loadTodo(c, tx)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	c.Set("todo", todo)
	return c.Render(http.StatusOK, r.HTML("todos/edit.plush.html"))
}

// TodosUpdate handles POST /todos/{todo_id} (any user).
func TodosUpdate(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	todo, err := loadTodo(c, tx)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	todo.Description = c.Param("description")
	todo.TodoDate = parseTodoDate(c.Param("todo_date"))
	todo.Recurrence = todoRecurrenceParam(c)

	verrs, err := tx.ValidateAndUpdate(todo)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("errors", verrs)
		c.Set("todo", todo)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("todos/edit.plush.html"))
	}

	c.Flash().Add("success", T.Translate(c, "todos.updated.success"))
	return c.Redirect(http.StatusSeeOther, "/todos")
}

// TodosDestroy handles POST /todos/{todo_id}/delete — admin only.
func TodosDestroy(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	todo, err := loadTodo(c, tx)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := tx.Destroy(todo); err != nil {
		return err
	}

	c.Flash().Add("success", T.Translate(c, "todos.deleted.success"))
	return c.Redirect(http.StatusSeeOther, "/todos")
}

// TodosDone handles POST /todos/{todo_id}/done — any user. Stores done_at
// (now) and done_by (current session user).
func TodosDone(c buffalo.Context) error {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	todo, err := loadTodo(c, tx)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if !todo.IsDone() {
		todo.DoneAt = nulls.NewTime(models.FormWallClockNow())
		todo.DoneByID = nulls.NewUUID(GetCurrentUser(c).ID)
		if err := tx.Save(todo); err != nil {
			return err
		}
		// Recurring todo (issue #199-8): spawn the next occurrence
		// (todo_date + 1 week/month/year) alongside the closed one.
		if next := todo.NextOccurrence(); next != nil {
			if err := tx.Create(next); err != nil {
				return err
			}
		}
	}

	c.Flash().Add("success", T.Translate(c, "todos.done.success"))
	return c.Redirect(http.StatusSeeOther, "%s", todoSafeRedirect(c.Param("redirect")))
}

// todoRecurrenceParam returns the whitelisted recurrence form value
// (issue #199-8); anything else maps to one-shot ("").
func todoRecurrenceParam(c buffalo.Context) string {
	switch c.Param("recurrence") {
	case models.TodoRecurrenceWeekly, models.TodoRecurrenceMonthly, models.TodoRecurrenceYearly:
		return c.Param("recurrence")
	}
	return models.TodoRecurrenceNone
}

// todoSafeRedirect returns target when it is a safe local path (starts with
// a single "/", anchors allowed), otherwise "/todos". Guards against open
// redirects via absolute URLs or protocol-relative "//host" values.
func todoSafeRedirect(target string) string {
	if strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//") {
		return target
	}
	return "/todos"
}

// TodosReopen handles POST /todos/{todo_id}/reopen — admin only, to fix
// accidental done markings.
func TodosReopen(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	todo, err := loadTodo(c, tx)
	if err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	todo.DoneAt = nulls.Time{}
	todo.DoneByID = nulls.UUID{}
	if err := tx.Save(todo); err != nil {
		return err
	}

	c.Flash().Add("success", T.Translate(c, "todos.reopened.success"))
	return c.Redirect(http.StatusSeeOther, "/todos")
}
