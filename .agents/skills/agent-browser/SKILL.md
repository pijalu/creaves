---
name: agent-browser
description: Test the Creaves web app with agent-browser. Use when validating pages, forms, language variants (fr/en-US/de/nl), or running E2E checks on the Creaves wildlife-care app. Covers login, language switching via the lang cookie, navigation, form submission, and checking for template errors / 500s. Triggers include "test creaves", "validate creaves pages", "check language variants", "E2E creaves".
category: action
inline: false
mode: coder
temperature: 0.2
tools:
  - Bash(agent-browser:*)
  - Bash(npx agent-browser:*)
---

# Creaves testing with agent-browser

Fast browser automation CLI for AI agents (Chrome via CDP, accessibility-tree
snapshots with `@eN` refs). Use this skill for all Creaves UI validation.

## App facts

- **URL**: `http://127.0.0.1:3000`
- **Login**: `admin` / `admin`
- **Language switching**: cookie `lang` — values `fr`, `en-US`, `de`, `nl`.
  Base templates are English; `*.plush.fr.html` / `*.plush.de.html` /
  `*.plush.nl.html` variants are resolved per language with fallback to base.
- **Dev server**: `buffalo dev` from `creaves/` (auto-rebuilds Go + assets).

## Core workflow

```bash
agent-browser open http://127.0.0.1:3000/auth/new
agent-browser snapshot -i            # get refs like @e1, @e2
agent-browser fill @e1 "admin"       # login
agent-browser fill @e2 "admin"
agent-browser click @e3              # submit
agent-browser wait --load networkidle
agent-browser snapshot -i            # re-snapshot after every navigation
```

The browser persists between commands (background daemon). Refs are stale
after any page change — always re-snapshot before interacting again.

## Language switching

The `/lang/?lang=de&url=...` route sets the `lang` cookie. To force a
language directly, set the cookie before loading a page:

```bash
agent-browser open http://127.0.0.1:3000/
agent-browser cookie set lang de
agent-browser open http://127.0.0.1:3000/animaltypes
agent-browser snapshot -i
```

(or use the in-app language dropdown via `langLinks` in the navbar).

## Testing checklist (de/nl variant validation)

For each language (`de`, `nl`, plus `fr`/base-English regression), verify:

1. **Auth**: login page renders; login succeeds with `admin`/`admin`.
2. **Dashboard**: `/` loads, HTTP 200, no template error in server log.
3. **Each domain** spot-check: `animaltypes`, `animals`, `discoveries`,
   `discoverers`, `intakes`, `cares`, `caretypes`, `outtakes`,
   `outtaketypes`, `treatments`, `drugs`, `veterinaryvisits`, `feeding`,
   `species`, `animalages`, `zones`, `traveltypes`, `travels`,
   `localities`, `subside_groups`, `native_statuses`, `entry_causes`,
   `users`, `logentries`, `config`, `event_streams`, `dashboard`,
   `auth/landing`, `maintenance`, `export`.
4. **No hardcoded English in de/nl output**: snapshot visible text and
   confirm headings/buttons/labels are German (de) or Dutch (nl).
5. **Forms submit**: on a list page, open `new`, fill minimal required
   fields, submit, expect success flash and no 500.
6. **No 404/500**: every visited URL returns 200; server log shows no
   template resolution errors.

## Full command reference

For anything beyond this workflow (screenshots, sessions, parallel runs,
auth state), load the canonical skill:

```bash
agent-browser skills get core --full
```
