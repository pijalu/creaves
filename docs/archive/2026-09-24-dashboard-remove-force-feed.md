# Dashboard — Remove “Animaux à gaver”

Date: 2026-09-24
Commit: `32a2d23` on `feature/depupdate`

## Request

Remove the `Animaux à gaver` force-feed preview section from the dashboard.

## Implementation

- Removed the force-feed heading, action, and preview table from French, English, German, and Dutch dashboard templates.
- Removed dashboard force-feed queries, context values, preview limit, and obsolete loader from `actions/dashboard.go`.
- Feeding-page force-feed data and workflow were not changed.

## Tests and validation

- `TestDashboardOmitsForceFeedSection` verifies the dashboard response contains neither the force-feed heading nor `animalsToForceFeed`, while existing dashboard content remains available.
- `go test -count=1 -race -cover ./...` passed; actions coverage 51.3%.
- `go vet ./...` and `staticcheck ./...` passed.
- `gocognit -over 15 .` and `gocyclo -over 12 .` reported only pre-existing repository-wide findings; no changed function in `actions/dashboard.go` appeared.
- Authenticated `agent-browser` checks at `/dashboard` in French, English, German, and Dutch found each localized existing dashboard heading and none of the removed force-feed headings.
- `agent-browser errors` returned no browser errors; console output contained only Buffalo live-reload notices.
