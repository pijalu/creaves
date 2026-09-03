package actions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
)

// ---------------------------------------------------------------------------
// Bug 1: registration mass-assignment privilege escalation
// ---------------------------------------------------------------------------

// newRegistrationTestApp builds a minimal Buffalo app exposing only the
// registration POST (UsersCreate) with the shared MySQL test database
// injected as "tx". No current_user is ever set, reproducing the anonymous
// self-registration flow.
func newRegistrationTestApp(t *testing.T) *buffalo.App {
	t.Helper()
	tx := searchTestDB(t)

	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			c.Set("tx", tx)
			return next(c)
		}
	})
	a.POST("/registration/", UsersCreate)
	return a
}

func registrationForm(vals url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/registration/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// TestUsersCreateRegistrationCannotSelfPromote posts a crafted registration
// form including privileged flags (Admin/Approved/Shared). Before the fix,
// c.Bind(u) accepted them and the row was created with admin=1 approved=1.
// After the fix, anonymous registrations must always land with all flags
// false.
func TestUsersCreateRegistrationCannotSelfPromote(t *testing.T) {
	a := newRegistrationTestApp(t)
	tx := searchTestDB(t)

	login := "sec_regress_evil"
	// ensure cleanup even on failure
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})
	tx.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()

	vals := url.Values{}
	vals.Set("Login", login)
	vals.Set("Password", "evilpass123")
	vals.Set("PasswordConfirmation", "evilpass123")
	vals.Set("Admin", "true")
	vals.Set("Approved", "true")
	vals.Set("Shared", "true")

	w := httptest.NewRecorder()
	a.ServeHTTP(w, registrationForm(vals))

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect after registration, got %d (body: %.200s)", w.Code, w.Body.String())
	}

	u := &models.User{}
	if err := tx.Where("login = ?", login).First(u); err != nil {
		t.Fatalf("user %q not created: %v", login, err)
	}
	if u.Admin {
		t.Error("mass-assignment regression: anonymous registration created user with admin=true")
	}
	if u.Approved {
		t.Error("mass-assignment regression: anonymous registration created user with approved=true")
	}
	if u.Shared {
		t.Error("mass-assignment regression: anonymous registration created user with shared=true")
	}
}

// TestUsersCreateAdminCanStillGrantFlags pins the legitimate flow: an
// authenticated admin creating a user via POST must be able to grant the
// privileged flags (the fix keys off current_user, not off the form).
func TestUsersCreateAdminCanStillGrantFlags(t *testing.T) {
	tx := searchTestDB(t)
	admin := &models.User{Login: "sec_regress_admin", Admin: true, Approved: true}
	admin.Password = "adminpass123"
	admin.PasswordConfirmation = "adminpass123"
	if _, err := admin.Create(tx); err != nil {
		t.Fatalf("failed to create admin fixture: %v", err)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM users WHERE login IN ('sec_regress_admin','sec_regress_granted')").Exec()
	})

	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			c.Set("tx", tx)
			c.Set("current_user", admin)
			return next(c)
		}
	})
	a.POST("/registration/", UsersCreate)

	vals := url.Values{}
	vals.Set("Login", "sec_regress_granted")
	vals.Set("Password", "userpass123")
	vals.Set("PasswordConfirmation", "userpass123")
	vals.Set("Admin", "true")
	vals.Set("Approved", "true")

	w := httptest.NewRecorder()
	a.ServeHTTP(w, registrationForm(vals))

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect after admin-created user, got %d (body: %.200s)", w.Code, w.Body.String())
	}

	u := &models.User{}
	if err := tx.Where("login = ?", "sec_regress_granted").First(u); err != nil {
		t.Fatalf("user not created: %v", err)
	}
	if !u.Admin || !u.Approved {
		t.Errorf("admin-granted flags lost: admin=%v approved=%v", u.Admin, u.Approved)
	}
}

// ---------------------------------------------------------------------------
// Bug 8: guest rate limiter must not trust spoofed X-Forwarded-For
// ---------------------------------------------------------------------------

func TestGuestClientIPIgnoresXFFWithoutTrustedProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")

	req := httptest.NewRequest(http.MethodGet, "/guest/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "6.6.6.6")

	if got := guestClientIP(req); got != "10.0.0.1" {
		t.Errorf("guestClientIP with no trusted proxies = %q, want %q (spoofed XFF must be ignored)", got, "10.0.0.1")
	}
}

func TestGuestClientIPIgnoresXFFFromUntrustedPeer(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "192.168.1.1")

	req := httptest.NewRequest(http.MethodGet, "/guest/", nil)
	req.RemoteAddr = "10.0.0.1:1234" // not in TRUSTED_PROXIES
	req.Header.Set("X-Forwarded-For", "6.6.6.6")

	if got := guestClientIP(req); got != "10.0.0.1" {
		t.Errorf("guestClientIP from untrusted peer = %q, want %q", got, "10.0.0.1")
	}
}

func TestGuestClientIPHonorsXFFFromTrustedProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "192.168.1.1, 10.0.0.1")

	req := httptest.NewRequest(http.MethodGet, "/guest/", nil)
	req.RemoteAddr = "10.0.0.1:1234" // trusted
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 10.0.0.1")

	if got := guestClientIP(req); got != "6.6.6.6" {
		t.Errorf("guestClientIP from trusted proxy = %q, want first XFF entry %q", got, "6.6.6.6")
	}
}

func TestGuestClientIPFallsBackToRemoteAddrWhenTrustedButNoXFF(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1")

	req := httptest.NewRequest(http.MethodGet, "/guest/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	if got := guestClientIP(req); got != "10.0.0.1" {
		t.Errorf("guestClientIP without XFF = %q, want %q", got, "10.0.0.1")
	}
}

// ---------------------------------------------------------------------------
// Bug 7: webhook batch-size / rate form values are clamped to sane ranges
// ---------------------------------------------------------------------------

// webhookLimitContext is a minimal buffalo.Context exposing only Param().
type webhookLimitContext struct {
	buffalo.Context
	params map[string]string
}

func (c *webhookLimitContext) Param(key string) string { return c.params[key] }

func TestParseWebhookLimitsClampsValues(t *testing.T) {
	cases := []struct {
		name              string
		batch, rate       string
		wantBatch, wantRt int
	}{
		{"defaults when empty", "", "", 1, 60},
		{"normal values", "50", "120", 50, 120},
		{"zero clamped to minimum", "0", "0", 1, 1},
		{"negative clamped to minimum", "-5", "-100", 1, 1},
		{"huge clamped to maximum", "99999", "999999", 100, 10000},
		{"garbage falls back to default", "abc", "xyz", 1, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &webhookLimitContext{params: map[string]string{
				"Settings.WebhookBatchSize": tc.batch,
				"Settings.WebhookMaxPerMin": tc.rate,
			}}
			b, m := parseWebhookLimits(c)
			if b != tc.wantBatch || m != tc.wantRt {
				t.Errorf("parseWebhookLimits(%q,%q) = (%d,%d), want (%d,%d)",
					tc.batch, tc.rate, b, m, tc.wantBatch, tc.wantRt)
			}
		})
	}
}
