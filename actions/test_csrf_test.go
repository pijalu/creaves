package actions

import (
	"net/http"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// testCSRFToken obtains a CSRF token bound to the session carried by client.
// The token is rendered on any HTML page (csrf-token meta tag); anonymous
// sessions get it from the login form. mw-csrf (gorilla/csrf) rejects unsafe
// methods (POST/PUT/DELETE) without a token, so test POSTs must carry one.
func testCSRFToken(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	for _, path := range []string{"/", "/auth/new"} {
		resp, err := client.Get(baseURL + path)
		require.NoError(t, err)
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, rerr)
		if resp.StatusCode == http.StatusOK {
			if m := csrfTokenRe.FindSubmatch(body); m != nil {
				return string(m[1])
			}
		}
	}
	t.Fatal("no CSRF token obtainable for this session")
	return ""
}

// setCSRFHeader stamps req with the X-CSRF-Token mw-csrf accepts for
// non-GET requests (equivalent to the authenticity_token form field).
func setCSRFHeader(t *testing.T, client *http.Client, baseURL string, req *http.Request) {
	t.Helper()
	req.Header.Set("X-CSRF-Token", testCSRFToken(t, client, baseURL))
}
