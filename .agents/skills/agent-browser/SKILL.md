# agent-browser validation

Use `agent-browser` CLI for authenticated browser checks.

## Setup

```sh
command -v agent-browser
agent-browser skills get core --full
```

## Workflow

```sh
agent-browser open http://127.0.0.1:3000/auth/new
agent-browser snapshot -i
agent-browser fill @e1 admin
agent-browser fill @e2 admin
agent-browser click @e3
agent-browser wait --load networkidle
agent-browser snapshot -c
```

Refs are invalid after navigation; always snapshot again. Use `get text`, `get html`,
`get url`, and `get count` for assertions. Close sessions with `agent-browser close`.

## Locale validation

Set locale through app language links or `/lang/?lang=de&url=/`. Open target page,
then assert localized reference values appear and canonical French values do not appear
where translations exist. Check both rendered text and JSON/XHR output when dynamic hints
are involved.

```sh
agent-browser open 'http://127.0.0.1:3000/lang/?lang=de&url=/'
agent-browser wait --load networkidle
agent-browser snapshot -c
agent-browser get text body
```

Never use screenshots as sole validation. Capture command output and URL as evidence.
