package actions

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFeedingIndexRedirect pins the §8.3 retirement contract of
// docs/care-expert.md: /feeding is gone — the route is kept for bookmarks
// but the handler redirects to the day plan filtered on feeding kind
// (GET /care_plan?kind=feeding). The former sort-order assertions of this
// test retired with the page itself (§197 sub-item 7 lived on /feeding).
func TestFeedingIndexRedirect(t *testing.T) {
	requireMySQLTestDB(t)

	client, baseURL := adminClientWithURL(t)
	// Stop at the first response so the redirect itself is observable.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(baseURL + "/feeding")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, "/care_plan?kind=feeding", resp.Header.Get("Location"))
}

// TestFeedingRedirectTargetRenders proves the redirect target (day plan,
// kind-filtered) serves 200 HTML for an admin session.
func TestFeedingRedirectTargetRenders(t *testing.T) {
	requireMySQLTestDB(t)

	client, baseURL := adminClientWithURL(t)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(baseURL + "/feeding")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	loc := resp.Header.Get("Location")
	require.Equal(t, "/care_plan?kind=feeding", loc)

	target, err := client.Get(baseURL + loc)
	require.NoError(t, err)
	defer target.Body.Close()
	require.Equal(t, http.StatusOK, target.StatusCode, "the redirect target /care_plan?kind=feeding must render")
}
