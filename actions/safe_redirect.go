package actions

import (
	"strings"

	"github.com/gobuffalo/buffalo"
)

// ---------------------------------------------------------------------------
// BUG-R5: the "back" form/query param is user-controlled and was used
// verbatim as a redirect target (open redirect — a crafted link could send
// users to an external phishing site after POST). safeBackParam returns the
// param only when it is a local path (starts with "/" but not "//", which
// would be scheme-relative); anything else falls back to "/".
// ---------------------------------------------------------------------------

// safeBackParam returns a safe local redirect target for the back param.
func safeBackParam(c buffalo.Context) string {
	return safeRedirectTarget(c.Param("back"))
}

// safeRedirectTarget filters a user-supplied redirect target down to local
// paths. Kept separate from safeBackParam so it is trivially unit-testable.
func safeRedirectTarget(target string) string {
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		return "/"
	}
	// A backslash can be interpreted as "/" by some browsers
	// ("/\evil.example" ≈ "//evil.example") — reject targets containing it.
	if strings.ContainsAny(target, "\\") {
		return "/"
	}
	return target
}

// localBackParam returns target when it is a safe local path, else "" —
// for LINK BUILDING (bugs.md R5-2d): an invalid back target is dropped so
// the link falls back to the plain self URL instead of silently chaining
// to the dashboard ("/").
func localBackParam(target string) string {
	if target == "" || !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		return ""
	}
	if strings.Contains(target, "\\") {
		return ""
	}
	return target
}
