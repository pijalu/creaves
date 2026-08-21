# Translation seed tooling

Regenerate embedded startup translation SQL after editing `translation_worklist.csv`:

```sh
python3 scripts/update_translation_seeds.py
```

Preview drift without changing files:

```sh
python3 scripts/update_translation_seeds.py --check
```

The tool updates existing `(table, record_id, field, locale)` rows in
`grifts/translations_{en-US,nl,de}.sql`. It preserves generated translation IDs
and does not invent values or add worklist rows missing from the startup seed
artifacts.
