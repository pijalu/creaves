# Event show delivery status fix (archive)

Date: 2026-09-04. Status: implemented.

## Problem

Event show page labeled `processed_at` simply “Processed”, which could misleadingly
suggest webhook delivery had completed. Local processing and remote delivery are
independent event lifecycle states.

## Fix

- Replaced ambiguous show-page processed row with separate **Delivered** status
  (`delivered_at`) and **Locally Processed** status (`processed_at`) rows.
- Added translations for both labels in English, French, German, and Dutch.

## Validation

- `git diff --check` — pass.
- `go test ./actions -run TestConfigTemplatesParsing -count=1` — pass.
- Browser verification: open an event URL with `delivered_at` set and confirm the
  Delivered badge is positive independently of Locally Processed.
