# Archive — #199-14 `/users/` admin page overhaul

**Status**: ✅ DONE — commit `7591c68` (branch `feature/open-issues-2026-10`, 2026-09-21)

## Observed
(a) Edit-user form still in English (untranslated). (b) Roles Admin/Maintainer/Shared were
separate checkboxes instead of a single "Account role" selector. (c) Edit buttons visible on
pages where the account type has no write rights. (d) `/users/` list showed "Admin" and
"Partagé" columns. (e) No filters on the users table.

## Root causes found
- (a) `templates/users/edit.plush.fr.html` included the **EN base** form partial
  (`partial("users/form.html")`) instead of `users/form.plush.fr.html`; the base
  `_aform.plush.html` even had hardcoded **French** strings ("Utilisateur", "Lecteur", …).
- (b) `_aform` had 4 checkboxes (Admin/Maintainer/Approved/Shared) + a separate `Role` select.
- (c) `show.plush.*` showed the edit button to Shared accounts although the Edit route 403s
  them (`cu.Shared ||` in `UsersResource.Edit/Update/Destroy`).
- Pre-existing security gap found while fixing (b): a crafted non-admin self-PUT could set
  `Admin=true` (only Role/volunteer flags were restored by `applyUserFieldPermissions`).

## Fix
- **(a) Localization**: `_form.plush*.html` + `_aform.plush*.html` + `edit/new/index.plush*.html`
  rewritten with `t()` keys; all 4 variants now structurally identical (parity-clean).
  50+ new keys in `locales/users.{en-us,fr,de,nl}.yaml`.
- **(b) Single selector**: one `<select name="AccountRole">` with values
  `"" / admin / maintainer / shared / lecteur / scientifique / spw` (maintainer option only
  visible to maintainers). Server-side `applyAccountRoleSelection` (actions/role_guard.go)
  folds the value onto the existing flag columns — no schema change; `userAccountRole`
  (registered plush helper) is the inverse for preselecting. Applied in
  `UsersResource.Create`, `UsersResource.Update` **and** `UsersCreate` (the admin "new user"
  page posts to the registration handler). `Approved` stays a separate checkbox.
  Update now snapshots the persisted row **before** zeroing flags and restores
  Admin/Shared/Maintainer for non-admin actors (closes the escalation gap; covered by
  `TestUsersUpdateNonAdminCannotEscalate`).
- **(c) Button gating**: show/index edit+destroy buttons now require
  `current_user.Admin && !current_user.Shared` (mirrors route authz). Self-edit button hidden
  for Shared accounts.
- **(d) Columns**: "Admin" and "Partagé" dropped from the index; the Role column shows the
  effective account role label via a t() if-chain (4 locales).
- **(e) Filters**: `usersListFilters` (actions/user_sort.go) adds `role`
  (admin/maintainer/shared/user/lecteur/scientifique/spw) and `status` (active/pending)
  query-param filters; UI selects in the index form; preserved across sort links.

## Tests
- `actions/users_account_role_test.go`: `TestApplyAccountRoleSelection`,
  `TestUserAccountRoleHelper`, `TestUsersUpdateAccountRoleSelector` (shared→lecteur→
  maintainer-gate→regular round-trip), `TestUsersUpdateNonAdminCannotEscalate`,
  `TestUsersCreateAccountRoleSelector`, `TestRegistrationAccountRoleSelector`,
  `TestUsersListFilters`. All pass; existing #107 suite green.
- e2e (agent-browser, admin): EN index headers `[Name Login Email City Role Approved]`,
  role/status selects present; role=shared filter → 17 rows all "Shared account" + Clear
  button; edit form fully localized EN + FR ("Rôle du compte", options "Utilisateur /
  Administrateur / Mainteneur / Compte partagé / Lecteur / Scientifique / SPW"); DE/NL index
  checked; selector round-trip on `accueil1` (shared→lecteur→shared, DB verified, restored);
  temp shared user `e2e_shared` created via UI (Shared=1) then curl-verified: own show page
  has no edit/destroy buttons, edit route 403, users list 403; user deleted afterwards.
- Gates: `go vet` ✓, `staticcheck` ✓, gocognit 80 (baseline), gocyclo 66 (baseline; Update
  16→19, already flagged), full suite: only the 2 pre-existing grifts failures.
