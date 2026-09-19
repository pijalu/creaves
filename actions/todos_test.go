package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

// postTodoForm posts a form to the given path with a CSRF token.
func postTodoForm(t *testing.T, client *http.Client, baseURL, path, token string, vals url.Values) *http.Response {
	t.Helper()
	vals.Set("authenticity_token", token)
	resp, err := client.PostForm(baseURL+path, vals)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// todoToken fetches the given page and extracts the CSRF token.
func todoToken(t *testing.T, client *http.Client, baseURL, page string) string {
	t.Helper()
	resp, err := client.Get(baseURL + page)
	if err != nil {
		t.Fatalf("GET %s: %v", page, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d", page, resp.StatusCode)
	}
	m := csrfTokenRe.FindSubmatch(body)
	if m == nil {
		t.Fatalf("no authenticity_token on %s", page)
	}
	return string(m[1])
}

func todoCleanup(t *testing.T, description string) {
	t.Helper()
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM todos WHERE description = ?", description).Exec()
	})
}

// TestTodosCreateDoneDelete covers the full lifecycle: any user creates,
// any user marks done (stores user + date), only admin deletes (issue #147).
func TestTodosCreateDoneDelete(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	adminLogin, adminPass := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, adminLogin, adminPass)

	description := "TS-todo-" + uuid.Must(uuid.NewV4()).String()[:8]
	todoCleanup(t, description)

	token := todoToken(t, client, baseURL, "/todos/new")
	resp := postTodoForm(t, client, baseURL, "/todos", token, url.Values{
		"description": {description},
		"todo_date":   {"2026-09-19T10:00"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /todos status = %d, want 303: %s", resp.StatusCode, body)
	}

	todo := &models.Todo{}
	if err := models.DB.Where("description = ?", description).First(todo); err != nil {
		t.Fatalf("todo not persisted: %v", err)
	}
	if todo.DoneAt.Valid {
		t.Error("fresh todo must not be done")
	}
	if todo.TodoDate.UTC().Format("2006-01-02 15:04") != "2026-09-19 08:00" {
		// DSN stores UTC: 10:00 local (CEST) == 08:00 UTC
		t.Errorf("todo_date = %s", todo.TodoDate)
	}

	// empty description → validation error, 422 re-render
	resp = postTodoForm(t, client, baseURL, "/todos", token, url.Values{
		"description": {"  "},
		"todo_date":   {"2026-09-19T10:00"},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("POST /todos empty description: status = %d, want 422", resp.StatusCode)
	}

	// done: stores done_at + done_by (current user)
	admin := &models.User{}
	if err := models.DB.Where("login = ?", adminLogin).First(admin); err != nil {
		t.Fatalf("admin user: %v", err)
	}
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/todos/%s/done", todo.ID), token, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("POST done status = %d, want 303", resp.StatusCode)
	}
	todoID := todo.ID
	todo = &models.Todo{}
	if err := models.DB.Find(todo, todoID); err != nil {
		t.Fatalf("reload todo: %v", err)
	}
	if !todo.DoneAt.Valid {
		t.Error("done_at must be set after POST done")
	}
	if !todo.DoneByID.Valid || todo.DoneByID.UUID != admin.ID {
		t.Errorf("done_by_id = %v, want %s", todo.DoneByID, admin.ID)
	}

	// delete allowed for admin
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/todos/%s/delete", todoID), token, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("admin delete status = %d, want 303", resp.StatusCode)
	}
	if err := models.DB.Where("description = ?", description).First(&models.Todo{}); err == nil {
		t.Error("todo must be deleted")
	}
}

// TestTodosDestroyReopenAdminOnly: non-admin cannot delete or reopen (issue #147).
func TestTodosDestroyReopenAdminOnly(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	description := "TS-todo-ro-" + uuid.Must(uuid.NewV4()).String()[:8]
	todoCleanup(t, description)

	adminLogin, adminPass := feedingGuideUser(t, true)
	aclient, abaseURL := feedingGuideLogin(t, adminLogin, adminPass)
	token := todoToken(t, aclient, abaseURL, "/todos/new")
	resp := postTodoForm(t, aclient, abaseURL, "/todos", token, url.Values{
		"description": {description},
		"todo_date":   {"2026-09-19T10:00"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("seed todo: status = %d", resp.StatusCode)
	}
	todo := &models.Todo{}
	if err := models.DB.Where("description = ?", description).First(todo); err != nil {
		t.Fatalf("seeded todo: %v", err)
	}
	// mark done as admin, then try to reopen/delete as plain user
	postTodoForm(t, aclient, abaseURL, fmt.Sprintf("/todos/%s/done", todo.ID), token, url.Values{})

	userLogin, userPass := feedingGuideUser(t, false)
	uclient, ubaseURL := feedingGuideLogin(t, userLogin, userPass)
	utoken := todoToken(t, uclient, ubaseURL, "/todos/new")

	resp = postTodoForm(t, uclient, ubaseURL, fmt.Sprintf("/todos/%s/delete", todo.ID), utoken, url.Values{})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin delete status = %d, want 403", resp.StatusCode)
	}
	resp = postTodoForm(t, uclient, ubaseURL, fmt.Sprintf("/todos/%s/reopen", todo.ID), utoken, url.Values{})
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin reopen status = %d, want 403", resp.StatusCode)
	}
	// non-admin CAN mark done
	resp = postTodoForm(t, uclient, ubaseURL, fmt.Sprintf("/todos/%s/done", todo.ID), utoken, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("non-admin done status = %d, want 303", resp.StatusCode)
	}

	// admin reopen clears done fields
	resp = postTodoForm(t, aclient, abaseURL, fmt.Sprintf("/todos/%s/reopen", todo.ID), token, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("admin reopen status = %d, want 303", resp.StatusCode)
	}
	todoID2 := todo.ID
	todo = &models.Todo{}
	if err := models.DB.Find(todo, todoID2); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if todo.DoneAt.Valid || todo.DoneByID.Valid {
		t.Errorf("reopened todo must have done fields cleared, got at=%v by=%v", todo.DoneAt, todo.DoneByID)
	}
}

// TestTodosIndexOrderingAndDashboard: index separates open (ascending) and
// done sections; dashboard block exposes open todos (issue #147).
func TestTodosIndexOrderingAndDashboard(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	descA := "TS-todo-A-" + uuid.Must(uuid.NewV4()).String()[:8]
	descB := "TS-todo-B-" + uuid.Must(uuid.NewV4()).String()[:8]
	descDone := "TS-todo-D-" + uuid.Must(uuid.NewV4()).String()[:8]
	todoCleanup(t, descA)
	todoCleanup(t, descB)
	todoCleanup(t, descDone)

	seed := func(description, date string) {
		t.Helper()
		td := &models.Todo{Description: description}
		td.TodoDate = parseTodoDate(date)
		if err := models.DB.Create(td); err != nil {
			t.Fatalf("seed todo %q: %v", description, err)
		}
	}
	seed(descB, "2026-03-05T09:00") // open, later
	seed(descA, "2026-01-02T08:00") // open, earlier → first
	seedDone(t, descDone)

	login, pass := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, pass)

	resp, err := client.Get(baseURL + "/todos")
	if err != nil {
		t.Fatalf("GET /todos: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /todos status = %d", resp.StatusCode)
	}
	s := string(body)
	for _, want := range []string{descA, descB, descDone} {
		if !strings.Contains(s, want) {
			t.Errorf("/todos page missing %q", want)
		}
	}
	// open section lists A before B
	if strings.Index(s, descA) > strings.Index(s, descB) {
		t.Errorf("open todos out of order: %q must appear before %q", descA, descB)
	}

	// dashboard renders the block with colors
	resp, err = client.Get(baseURL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /dashboard status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), descA) {
		t.Errorf("dashboard missing open todo %q", descA)
	}
}

func seedDone(t *testing.T, description string) {
	t.Helper()
	td := &models.Todo{Description: description}
	td.TodoDate = parseTodoDate("2026-02-02T08:00")
	td.DoneAt = nulls.NewTime(parseTodoDate("2026-02-03T09:00"))
	if err := models.DB.Create(td); err != nil {
		t.Fatalf("seed done todo: %v", err)
	}
}
