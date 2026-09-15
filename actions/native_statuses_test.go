package actions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/stretchr/testify/require"
)

// newNativeStatusTestApp builds a minimal app with the shared test DB and an
// optional current_user, registering the native-status resource routes plus
// the delete-with-replacement endpoints.
func newNativeStatusTestApp(t *testing.T, u *models.User) *buffalo.App {
	t.Helper()
	tx := searchTestDB(t)
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			c.Set("tx", tx)
			if u != nil {
				c.Set("current_user", u)
			}
			return next(c)
		}
	})
	res := NativeStatusesResource{}
	a.GET("/native_statuses/", res.List)
	a.GET("/native_statuses/new", res.New)
	a.POST("/native_statuses/", res.Create)
	a.GET("/native_statuses/{native_status_id}", res.Show)
	a.GET("/native_statuses/{native_status_id}/edit", res.Edit)
	a.PUT("/native_statuses/{native_status_id}", res.Update)
	a.DELETE("/native_statuses/{native_status_id}", res.Destroy)
	a.GET("/native_statuses/{native_status_id}/delete", NativeStatusDeleteNew)
	a.POST("/native_statuses/{native_status_id}/delete", NativeStatusDeleteCreate)
	return a
}

func newJSONRequest(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	r.Header.Set("Accept", "application/json")
	return r
}

// TestNativeStatusesMaintainerOnly pins that every native-status route
// requires the maintainer flag: plain admins get 403, maintainers pass.
func TestNativeStatusesMaintainerOnly(t *testing.T) {
	plainAdmin := &models.User{Login: "ns_plain_admin", Admin: true, Approved: true}
	maintainer := &models.User{Login: "ns_maintainer", Admin: true, Maintainer: true, Approved: true}

	routes := []struct {
		name   string
		method string
		path   string
	}{
		{"list", http.MethodGet, "/native_statuses/"},
		{"new", http.MethodGet, "/native_statuses/new"},
		{"show", http.MethodGet, "/native_statuses/NS1"},
		{"edit", http.MethodGet, "/native_statuses/NS1/edit"},
		{"delete-new", http.MethodGet, "/native_statuses/NS1/delete"},
		{"destroy", http.MethodDelete, "/native_statuses/NS1"},
	}

	for _, tc := range routes {
		t.Run("plain-admin-"+tc.name, func(t *testing.T) {
			a := newNativeStatusTestApp(t, plainAdmin)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, newJSONRequest(tc.method, tc.path, ""))
			require.Equal(t, http.StatusForbidden, w.Code,
				"plain admin must get 403 on %s %s", tc.method, tc.path)
		})
	}

	// Maintainer reaches the list (seeded via test DB setup).
	a := newNativeStatusTestApp(t, maintainer)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, newJSONRequest(http.MethodGet, "/native_statuses/", ""))
	require.Equal(t, http.StatusOK, w.Code, "maintainer must get 200 on list")
}

// nativeStatusTestFixture creates two native statuses and one species using
// the first, returning cleanup.
func nativeStatusTestFixture(t *testing.T) {
	t.Helper()
	tx := searchTestDB(t)
	now := "NOW()"
	for _, q := range []string{
		"INSERT INTO native_statuses (id, status, indication, freeable, created_at, updated_at) VALUES ('NST_A', 'NST A', 'x', 0, " + now + ", " + now + ") ON DUPLICATE KEY UPDATE status = VALUES(status)",
		"INSERT INTO native_statuses (id, status, indication, freeable, created_at, updated_at) VALUES ('NST_B', 'NST B', 'x', 0, " + now + ", " + now + ") ON DUPLICATE KEY UPDATE status = VALUES(status)",
		"DELETE FROM species WHERE id = 'NST_SP'",
		"INSERT INTO species (id, species, creaves_species, class, `order`, family, native_status, agw_group, subside_group, game, huntable, created_at, updated_at) VALUES ('NST_SP', 'NST Species', 'NST Species', 'C', 'O', 'F', 'NST_A', 'G', 'S', 0, 0, " + now + ", " + now + ")",
	} {
		require.NoError(t, tx.RawQuery(q).Exec(), "fixture query failed: %s", q)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE id = 'NST_SP'").Exec()
		tx.RawQuery("DELETE FROM native_statuses WHERE id IN ('NST_A','NST_B')").Exec()
	})
}

// TestNativeStatusDestroyBlockedWhenUsed: direct DELETE must be refused with
// 422 while species reference the record.
func TestNativeStatusDestroyBlockedWhenUsed(t *testing.T) {
	nativeStatusTestFixture(t)
	maintainer := &models.User{Login: "ns_maintainer2", Admin: true, Maintainer: true, Approved: true}

	a := newNativeStatusTestApp(t, maintainer)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, newJSONRequest(http.MethodDelete, "/native_statuses/NST_A", ""))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code, "destroy of in-use native status must be refused")

	// Record must still exist.
	tx := searchTestDB(t)
	var count int
	require.NoError(t, tx.RawQuery("SELECT COUNT(*) FROM native_statuses WHERE id = 'NST_A'").First(&count))
	require.Equal(t, 1, count)
}

// TestNativeStatusDeleteCreateRequiresReplacement: POST without replacement
// must fail with 422 when the record is used by species.
func TestNativeStatusDeleteCreateRequiresReplacement(t *testing.T) {
	nativeStatusTestFixture(t)
	maintainer := &models.User{Login: "ns_maintainer3", Admin: true, Maintainer: true, Approved: true}

	a := newNativeStatusTestApp(t, maintainer)
	w := httptest.NewRecorder()
	a.ServeHTTP(w, newJSONRequest(http.MethodPost, "/native_statuses/NST_A/delete", url.Values{"replacement_id": {""}}.Encode()))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)

	// Invalid replacement id is rejected too.
	w2 := httptest.NewRecorder()
	a.ServeHTTP(w2, newJSONRequest(http.MethodPost, "/native_statuses/NST_A/delete", url.Values{"replacement_id": {"NOPE"}}.Encode()))
	require.Equal(t, http.StatusBadRequest, w2.Code)

	// Same id as source is rejected.
	w3 := httptest.NewRecorder()
	a.ServeHTTP(w3, newJSONRequest(http.MethodPost, "/native_statuses/NST_A/delete", url.Values{"replacement_id": {"NST_A"}}.Encode()))
	require.Equal(t, http.StatusBadRequest, w3.Code)
}

// TestNativeStatusDeleteWithReplacement: species must be remapped to the
// replacement and the source record deleted.
func TestNativeStatusDeleteWithReplacement(t *testing.T) {
	nativeStatusTestFixture(t)
	maintainer := &models.User{Login: "ns_maintainer4", Admin: true, Maintainer: true, Approved: true}

	a := newNativeStatusTestApp(t, maintainer)
	w := httptest.NewRecorder()
	req := newJSONRequest(http.MethodPost, "/native_statuses/NST_A/delete", url.Values{"replacement_id": {"NST_B"}}.Encode())
	// HTML flow: no JSON Accept so redirects are fine.
	req.Header.Del("Accept")
	a.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code, "expected redirect after delete, got %d", w.Code)

	tx := searchTestDB(t)
	var ns string
	require.NoError(t, tx.RawQuery("SELECT native_status FROM species WHERE id = 'NST_SP'").First(&ns))
	require.Equal(t, "NST_B", ns, "species must be remapped to the replacement")

	var count int
	require.NoError(t, tx.RawQuery("SELECT COUNT(*) FROM native_statuses WHERE id = 'NST_A'").First(&count))
	require.Equal(t, 0, count, "source native status must be deleted")
}

// TestNativeStatusDeleteUnusedWithoutReplacement: unused record can be
// deleted through the flow without a replacement.
func TestNativeStatusDeleteUnusedWithoutReplacement(t *testing.T) {
	nativeStatusTestFixture(t)
	maintainer := &models.User{Login: "ns_maintainer5", Admin: true, Maintainer: true, Approved: true}

	a := newNativeStatusTestApp(t, maintainer)
	w := httptest.NewRecorder()
	req := newJSONRequest(http.MethodPost, "/native_statuses/NST_B/delete", url.Values{"replacement_id": {""}}.Encode())
	req.Header.Del("Accept")
	a.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)

	tx := searchTestDB(t)
	var count int
	require.NoError(t, tx.RawQuery("SELECT COUNT(*) FROM native_statuses WHERE id = 'NST_B'").First(&count))
	require.Equal(t, 0, count)
}
