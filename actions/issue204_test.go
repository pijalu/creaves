package actions

import (
	"io"
	"net/http"
	"testing"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// TestIssue204NewEntryShortcutsWithoutAnimal verifies menu shortcuts open the
// animal-selection step instead of evaluating Value() on an integer field and
// rendering the error page.
func TestIssue204NewEntryShortcutsWithoutAnimal(t *testing.T) {
	requireMySQLTestDB(t)
	client, baseURL := adminClientWithURL(t)

	tests := []struct {
		path string
		want string
	}{
		{path: "/treatments/new", want: "Select animal for treatment"},
		{path: "/veterinaryvisits/new", want: "Select animal for veterinary visit"},
		{path: "/travels/new", want: "Select animal linked to the travel"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := client.Get(baseURL + tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			got := string(body)
			require.Equal(t, http.StatusOK, resp.StatusCode, got)
			require.Contains(t, got, tt.want)
			require.NotContains(t, got, "0/0")
			require.NotContains(t, got, "reflect.Value.MethodByName")
		})
	}
}

// TestDashboardOmitsForceFeedSection verifies the force-feed section requested
// for removal is absent from the dashboard.
func TestDashboardOmitsForceFeedSection(t *testing.T) {
	requireMySQLTestDB(t)
	client, baseURL := adminClientWithURL(t)

	code, body := roleTestGetBody(t, client, baseURL, "/dashboard")
	require.Equal(t, http.StatusOK, code)
	require.NotContains(t, body, "Animals to force feed")
	require.NotContains(t, body, "animalsToForceFeed")
}

func TestStayDurationBucket(t *testing.T) {
	labels := []string{"<12H", "12-24H", "24-48H", "+48H"}
	tests := []struct {
		hours int
		want  string
	}{
		{hours: 0, want: labels[0]},
		{hours: 11, want: labels[0]},
		{hours: 12, want: labels[1]},
		{hours: 23, want: labels[1]},
		{hours: 24, want: labels[2]},
		{hours: 47, want: labels[2]},
		{hours: 48, want: labels[3]},
	}
	for _, tt := range tests {
		got := stayDurationBucket(nulls.NewInt(tt.hours), labels[0], labels[1], labels[2], labels[3])
		require.Equal(t, tt.want, got)
	}
	require.Empty(t, stayDurationBucket(nulls.Int{}, labels[0], labels[1], labels[2], labels[3]))
}
