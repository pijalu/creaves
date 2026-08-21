#!/usr/bin/env python3
"""Update embedded translation seed SQL from translation_worklist.csv.

Only keys already present in each seed artifact are updated. Extra worklist rows
remain available for later migrations but are not added to startup artifacts.
"""
import argparse
import csv
import re
import sys
from pathlib import Path

LOCALES = ("en-US", "nl", "de")
VALUE_RE = re.compile(
    r"VALUES \('[^']*','([^']*)','([^']*)','([^']*)','([^']*)','((?:\\.|''|[^'\\])*)',NOW\(\),NOW\(\)\);"
)

def sql_escape(value):
    return value.replace("\\", "\\\\").replace("'", "\\'").replace("\n", "\\n").replace("\r", "\\r")


def load_worklist(path):
    with path.open(newline="", encoding="utf-8-sig") as stream:
        rows = csv.DictReader(stream)
        required = {"table", "record_id", "field", *LOCALES}
        if not required.issubset(rows.fieldnames or ()):
            raise ValueError("worklist missing columns: " + ", ".join(sorted(required)))
        return {(row["table"], row["record_id"], row["field"], locale): row[locale]
                for row in rows for locale in LOCALES if row[locale]}


def update_file(path, values, check):
    original = path.read_text(encoding="utf-8")
    changed = 0
    seen = set()

    def replace(match):
        nonlocal changed
        table, record_id, field, locale, current = match.groups()
        key = (table, record_id, field, locale)
        seen.add(key)
        wanted = values.get(key)
        if wanted is None:
            return match.group(0)
        escaped = sql_escape(wanted)
        if escaped == current:
            return match.group(0)
        changed += 1
        return match.group(0).replace("'" + current + "'", "'" + escaped + "'", 1)

    updated = VALUE_RE.sub(replace, original)
    if not check and updated != original:
        path.write_text(updated, encoding="utf-8")
    return changed, seen


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--worklist", type=Path, default=Path("translation_worklist.csv"))
    parser.add_argument("--seed-dir", type=Path, default=Path("grifts"))
    parser.add_argument("--check", action="store_true", help="report drift without writing files")
    args = parser.parse_args()
    values = load_worklist(args.worklist)
    total = 0
    for locale in LOCALES:
        path = args.seed_dir / ("translations_" + locale + ".sql")
        changed, _ = update_file(path, values, args.check)
        total += changed
        print(locale + ": " + str(changed) + " updates" + (" needed" if args.check else " written"))
    if args.check and total:
        return 1
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        print("error: " + str(error), file=sys.stderr)
        sys.exit(2)