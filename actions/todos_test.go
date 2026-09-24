package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

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
	if todo.TodoDate.UTC().Format("2006-01-02 15:04") != "2026-09-19 10:00" {
		// The picked wall clock must round-trip verbatim: the DSN stores the
		// UTC wall clock in the naive DATETIME column, and parsing the form
		// value in the UTC frame keeps the displayed time in local time
		// (bugs.md calendar item: todos displayed 2h earlier in CEST).
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

// TestTodosRecurrence (issue #199-8): a recurring todo spawns the next
// occurrence (todo_date + 1 week/month/year) when marked done; a
// one-shot todo does not.
func TestTodosRecurrence(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	adminLogin, adminPass := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, adminLogin, adminPass)

	description := "TS-todo-rec-" + uuid.Must(uuid.NewV4()).String()[:8]
	todoCleanup(t, description)

	token := todoToken(t, client, baseURL, "/todos/new")
	resp := postTodoForm(t, client, baseURL, "/todos", token, url.Values{
		"description": {description},
		"todo_date":   {"2026-09-19T10:00"},
		"recurrence":  {"weekly"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /todos status = %d, want 303: %s", resp.StatusCode, body)
	}

	todo := &models.Todo{}
	if err := models.DB.Where("description = ?", description).First(todo); err != nil {
		t.Fatalf("todo not persisted: %v", err)
	}
	if todo.Recurrence != "weekly" {
		t.Fatalf("recurrence = %q, want weekly", todo.Recurrence)
	}

	// mark done → next occurrence must exist with todo_date + 7 days,
	// not done, same recurrence
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/todos/%s/done", todo.ID), token, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST done status = %d, want 303", resp.StatusCode)
	}

	todos := []models.Todo{}
	if err := models.DB.Where("description = ?", description).Order("todo_date asc").All(&todos); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(todos) != 2 {
		t.Fatalf("todos with description = %d, want 2 (closed + next occurrence)", len(todos))
	}
	first, next := todos[0], todos[1]
	if !first.IsDone() {
		t.Error("original todo must be done")
	}
	if next.IsDone() {
		t.Error("spawned occurrence must be open")
	}
	if next.Recurrence != "weekly" {
		t.Errorf("spawned recurrence = %q, want weekly", next.Recurrence)
	}
	want := first.TodoDate.AddDate(0, 0, 7)
	if !next.TodoDate.Equal(want) {
		t.Errorf("spawned todo_date = %s, want %s (+7 days)", next.TodoDate, want)
	}

	// invalid recurrence value is normalized to one-shot
	description2 := description + "-x"
	todoCleanup(t, description2)
	resp = postTodoForm(t, client, baseURL, "/todos", token, url.Values{
		"description": {description2},
		"todo_date":   {"2026-09-19T10:00"},
		"recurrence":  {"daily"},
	})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST /todos (invalid recurrence) status = %d, want 303", resp.StatusCode)
	}
	oneShot := &models.Todo{}
	if err := models.DB.Where("description = ?", description2).First(oneShot); err != nil {
		t.Fatalf("one-shot todo not persisted: %v", err)
	}
	if oneShot.Recurrence != "" {
		t.Errorf("invalid recurrence must map to one-shot, got %q", oneShot.Recurrence)
	}
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/todos/%s/done", oneShot.ID), token, url.Values{})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("POST done (one-shot) status = %d, want 303", resp.StatusCode)
	}
	count, err := models.DB.Where("description = ?", description2).Count(&models.Todo{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("one-shot done must not spawn an occurrence, count = %d", count)
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

// getPage fetches a page with the logged-in client and returns the body.
func todoGetBody(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d", url, resp.StatusCode)
	}
	return string(body)
}

// TestTodosDueSoonFiltering (bugs.md TODO item): the dashboard lists only
// open todos due within the next 8h (or overdue); /todos splits open todos
// into the main list and a collapsed "later" section for todos due further
// out. Nothing is auto-marked done.
func TestTodosDueSoonFiltering(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	descOverdue := "TS-todo-ov-" + uuid.Must(uuid.NewV4()).String()[:8]
	descSoon := "TS-todo-so-" + uuid.Must(uuid.NewV4()).String()[:8]
	descLater := "TS-todo-la-" + uuid.Must(uuid.NewV4()).String()[:8]
	todoCleanup(t, descOverdue)
	todoCleanup(t, descSoon)
	todoCleanup(t, descLater)

	now := models.FormWallClockNow()
	seed := func(description string, date time.Time) {
		t.Helper()
		td := &models.Todo{Description: description, TodoDate: date}
		if err := models.DB.Create(td); err != nil {
			t.Fatalf("seed todo %q: %v", description, err)
		}
	}
	seed(descOverdue, now.Add(-2*time.Hour))
	seed(descSoon, now.Add(2*time.Hour))
	seed(descLater, now.Add(48*time.Hour))

	login, pass := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, pass)

	// dashboard: overdue + due-soon listed, later one hidden
	s := todoGetBody(t, client, baseURL+"/dashboard")
	for _, want := range []string{descOverdue, descSoon} {
		if !strings.Contains(s, want) {
			t.Errorf("dashboard missing due todo %q", want)
		}
	}
	if strings.Contains(s, descLater) {
		t.Errorf("dashboard must not list todo %q due in 48h", descLater)
	}

	// /todos: later todo present but inside the collapsed later section
	assertTodosIndexSplit(t, todoGetBody(t, client, baseURL+"/todos"), descOverdue, descSoon, descLater)

	// later todo must still be open in the DB (not auto-marked done)
	td := &models.Todo{}
	if err := models.DB.Where("description = ?", descLater).First(td); err != nil {
		t.Fatalf("reload later todo: %v", err)
	}
	if td.IsDone() {
		t.Error("later todo must NOT be auto-marked done")
	}
}

// assertTodosIndexSplit checks the /todos page split: all three todos
// listed, the due ones in the main section, the later one inside the
// collapsed laterTodosCollapse section.
func assertTodosIndexSplit(t *testing.T, s, descOverdue, descSoon, descLater string) {
	t.Helper()
	for _, want := range []string{descOverdue, descSoon, descLater} {
		if !strings.Contains(s, want) {
			t.Errorf("/todos missing %q", want)
		}
	}
	laterIdx := strings.Index(s, "laterTodosCollapse")
	if laterIdx < 0 {
		t.Fatal("/todos missing the collapsed later section")
	}
	if strings.Index(s, descLater) < laterIdx {
		t.Errorf("later todo %q must appear inside the collapsed later section", descLater)
	}
	// due todos must stay in the main (open) section, before the later section
	if strings.Index(s, descOverdue) > laterIdx || strings.Index(s, descSoon) > laterIdx {
		t.Error("due todos must stay in the main open section above the later collapse")
	}
}

// TestTodosDoneRedirect: the done endpoint honors a safe local `redirect`
// param (dashboard posts /#todos) and rejects absolute/protocol-relative
// targets (open-redirect guard), defaulting to /todos.
func TestTodosDoneRedirect(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	login, pass := feedingGuideUser(t, true)
	baseClient, baseURL := feedingGuideLogin(t, login, pass)
	// do not follow redirects: assert the Location header directly
	client := *baseClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	token := todoToken(t, &client, baseURL, "/todos/new")

	mkTodo := func() *models.Todo {
		t.Helper()
		description := "TS-todo-rd-" + uuid.Must(uuid.NewV4()).String()[:8]
		todoCleanup(t, description)
		td := &models.Todo{Description: description}
		td.TodoDate = parseTodoDate("2026-03-03T09:00")
		if err := models.DB.Create(td); err != nil {
			t.Fatalf("seed todo: %v", err)
		}
		return td
	}

	cases := []struct {
		name     string
		redirect string
		want     string
	}{
		{"dashboard anchor", "/#todos", "/#todos"},
		{"local path", "/todos", "/todos"},
		{"empty falls back", "", "/todos"},
		{"absolute URL rejected", "https://evil.example.com/x", "/todos"},
		{"protocol-relative rejected", "//evil.example.com/x", "/todos"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			todo := mkTodo()
			vals := url.Values{}
			if tc.redirect != "" {
				vals.Set("redirect", tc.redirect)
			}
			resp := postTodoForm(t, &client, baseURL, fmt.Sprintf("/todos/%s/done", todo.ID), token, vals)
			if resp.StatusCode != http.StatusSeeOther {
				t.Fatalf("done status = %d, want 303", resp.StatusCode)
			}
			if loc := resp.Header.Get("Location"); loc != tc.want {
				t.Errorf("Location = %q, want %q", loc, tc.want)
			}
		})
	}
}

// TestParseTodoDateWallClock: the picked wall clock must survive parsing
// verbatim (bugs.md calendar item). The DSN has no loc parameter, so the
// driver stores/loads the UTC wall clock of the naive DATETIME column;
// parsing in the UTC frame keeps the displayed time equal to the picked
// local time regardless of the server timezone.
func TestParseTodoDateWallClock(t *testing.T) {
	for _, raw := range []string{"2026-09-19T10:00", "2026-09-19T10:00:30", "2026-09-19 10:00", "2026-09-19"} {
		got := parseTodoDate(raw)
		if got.Location() != time.UTC {
			t.Errorf("parseTodoDate(%q) location = %v, want UTC", raw, got.Location())
		}
	}
	got := parseTodoDate("2026-09-19T10:00")
	if got.Format("2006-01-02 15:04") != "2026-09-19 10:00" {
		t.Errorf("parseTodoDate wall clock = %s, want 2026-09-19 10:00", got.Format("2006-01-02 15:04"))
	}
	// empty/unparsable falls back to the wall-clock "now" (UTC frame)
	fallback := parseTodoDate("")
	if fallback.Location() != time.UTC {
		t.Errorf("fallback location = %v, want UTC", fallback.Location())
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
