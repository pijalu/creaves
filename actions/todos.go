package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// TODO list (issue #147). Any user can create/edit/update todos and mark
// them done; only admins can delete or reopen.

// parseTodoDate parses the datetime-local form value with tolerant layouts,
// defaulting to now when empty or unparsable. Parsed in the server's local
// timezone (time.Parse would assume UTC and shift dates by the UTC offset).
func parseTodoDate(raw string) time.Time {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t
		}
	}
	return time.Now()
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
	now := time.Now()
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

// TodosIndex handles GET /todos: open todos first (todo_date ascending),
// then the done section (most recently completed first).
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

	openViews, err := todoViews(tx, *open)
	if err != nil {
		return err
	}
	doneViews, err := todoViews(tx, *done)
	if err != nil {
		return err
	}

	c.Set("openTodos", openViews)
	c.Set("doneTodos", doneViews)
	c.Set("isAdmin", GetCurrentUser(c).Admin)

	return c.Render(http.StatusOK, r.HTML("todos/todos.plush.html"))
}

// TodosNew handles GET /todos/new: empty form, todo_date defaults to now.
func TodosNew(c buffalo.Context) error {
	todo := &models.Todo{TodoDate: time.Now()}
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
		todo.DoneAt = nulls.NewTime(time.Now())
		todo.DoneByID = nulls.NewUUID(GetCurrentUser(c).ID)
		if err := tx.Save(todo); err != nil {
			return err
		}
	}

	c.Flash().Add("success", T.Translate(c, "todos.done.success"))
	return c.Redirect(http.StatusSeeOther, "/todos")
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
