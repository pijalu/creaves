# Fix archive — 2026-09-04 — console instance auth and show page

## Bugs closed

- Restricted webhook API keys now define canonical instance routing. Incoming instance IDs compare case-insensitively, then persist using the key's stored identifier. Envelope and event mismatches are rejected.
- Creaves webhook delivery no longer assumes acceptance for structured responses reporting errors or partial processing. Invalid response JSON also leaves events pending.
- Localized instance show templates contain renderable content directly, avoiding Buffalo's locale partial lookup (`instances/_show.plush.html`) failure.

## Validation

- `CGO_ENABLED=1 go test -tags sqlite ./actions/... ./models/...` in `creaves-console`
- `go test ./actions/... ./models/...` in `creaves`
- Added regression coverage for case-insensitive restricted-key routing and explicit partial webhook responses.
