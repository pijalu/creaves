## #147 — Liste des tâches à faire (TODOs) — SPEC RECEIVED, CLEAR
**Source:** https://github.com/pijalu/creaves/issues/147
**Spec (from reporter):**
- TODO list with simple items (short description).
- **Todo view (open items)**: users click to **confirm done** → stores **username + date of completion**.
- **Full CRUD** to manage todos, dedicated menu entry **"TODOs"** + a block on the **dashboard**.
- Fields: creation date; **todo date** (default: now); description; done date; done by (user).
- **Dashboard color code**: done **or** future → **green**; open, todo date within last 8h → **yellow**; open, todo date older than 8h → **red**.
- **Only admin can delete** a todo.
**Plan:**
1. **Migration (additive)**: `todos` table — `id` UUID PK, `description` TEXT NOT NULL, `todo_date` DATETIME NOT NULL (default now), `done_at` DATETIME NULL, `done_by_id` CHAR(36) NULL → users, `created_at`/`updated_at`. Creation date = `created_at`. Down migration drops table (new table, no prod data risk).
2. **Model** `models/todo.go` + validations (description presence); helper `Status(now)` returning green/yellow/red classification per spec.
3. **Resource** `actions/todos.go` mounted `/todos`: index (open first, done section), new/create (any user), edit/update (any user), destroy (**admin only** — check existing admin guard pattern), plus `POST /todos/{todo_id}/done` marking `done_at=now, done_by=current user`; optional `POST /todos/{todo_id}/reopen` (admin) to fix mistakes.
4. **Views** `templates/todos/` (4 locales): list with color badges, form (description + todo date datetime picker, default now), done button with confirm.
5. **Menu**: "TODOs" entry in nav (all locales); **dashboard block** listing open todos with color coding per spec.
6. **i18n**: `locales/todos.{fr,en-us,de,nl}.yaml` + dashboard keys.
**Test:**
- Unit: model validation; status classification (done/future→green; open ≤8h→yellow; open >8h→red boundary cases); done action stores user+date; destroy denied for non-admin, allowed for admin.
- e2e (agent-browser): create todo → appears green/yellow in dashboard → mark done (stores username+date) → admin deletes.
**Validation:** `go test ./...`; agent-browser flows; 4 locales; menu + dashboard verified.
**DONE ✅ commit 0ed5fa1** — todos table (NOT NULL enforced via sql() ALTER — fizz create_table ignores `null:false`, cf. attachments); Todo model w/ Status(now) + TableName() pin (pop pluralizes to "todoes"); /todos CRUD (any user) + admin-only delete/reopen; done stores session user+now; dashboard block w/ badges; menu ×4; templates ×4; handler+model tests; e2e: create/default-now, 4 badge colors live (green/yellow/red/future), ordering, done+reopen+delete. Gotchas found: fizz `drop_column` (NOT remove_column) is the only drop identifier in fizz v1.14.4 — the #175 down migration `add_stay_duration_to_outakes.down.fizz` is BROKEN the same way (pre-existing, unfixed); `time.Parse` UTC shift — datetime-local values must use ParseInLocation(time.Local) (fixed in #149 destination_at too, d7c476c); MySQL DSN has no loc= → all reads UTC (app-wide display convention, not changed).
