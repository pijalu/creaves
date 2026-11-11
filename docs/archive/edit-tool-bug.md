# Bug Report: `edit` tool — spurious `not_found`, indentation mangling, silent no-ops

| | |
|---|---|
| **Date** | 2026-09-25 |
| **Tool surface** | `edit` (search/replace file editor: `replace`, `replace_pattern`, `replace_lines`, `insert_after`, `insert_before`, `delete_lines`), plus adjacent `write` / `read` behavior |
| **Environment** | macOS (darwin), zsh shell; target file: UTF-8 Markdown spec, ~1600–1700 lines / ~107 KB, heavy multi-byte content (French accents, `§`, `—`, `’`, emoji `🔒 ✅ 🦔`, box-drawing chars `│ ┌ ┐`), some single lines >1 KB |
| **Overall impact** | Two full edit batches silently lost; one file corruption requiring manual repair; numerous failed legitimate edits. Only mitigated by adopting a verify-after-every-edit protocol (`git diff`, mtime checks, region re-reads). |

## Summary of issues

| # | Issue | Severity | Status |
|---|-------|----------|--------|
| 1 | Content `replace` reports `not_found` despite byte-exact anchor | High | Reproduced (this session) |
| 2 | Mixing line-numbered ops and content ops in one batch corrupts the file | High | Observed (earlier session) |
| 3 | Silent rollback: batch reports success, nothing persisted | **Critical** | Observed 2× (earlier session) |
| 4 | Default `indent_mode` rewrites caller-supplied indentation | Medium | Reproduced 3× (this session) |
| 5 | `replace_lines` rejects empty `new_content` | Low | Reproduced |
| 6 | `replace_pattern` replaced a whole line, dropping a prefix | Medium | Observed (earlier session) |
| 7 | `write` is not whole-file replace — partial/merge with misleading diff | Medium | Observed (earlier session) |

---

## Issue 1 — Content `replace` spuriously reports `not_found` (byte-exact anchor verified)

**Severity: High** — blocks legitimate edits and misdiagnoses the cause, sending the caller down a wrong debugging path.

### Actual behavior

Two consecutive `replace` attempts against line 744 of `docs/care-expert.md` failed:

```
edit 1/2 (replace): Text "each item is an actionable card with **A..." not found in
/Users/muaddib/dev/creaves.project/creaves/docs/care-expert.md (tried exact, trailing
whitespace, and fuzzy matching) — 0/1 lines of old_string matched the current file —
no changes were written; fix the failing edit and retry the whole batch
Hint: The file has drifted from your last read (see the line-match count above).
Re-read the target region with 'read' first, then retry with a SMALLER edit ...
```

### Verification that the anchor was byte-exact

The anchor substring was extracted from the file itself and compared:

```bash
$ awk 'NR==744' docs/care-expert.md | grep -oE "each item is an actionable card.{0,110}" | cat -v
each item is an actionable card with **Apply**/**Defer**/**Skip** one-click buttons, animal name + cage, action description, and status badge
```

Output is byte-identical to the rejected `old_string` (pure ASCII, no hidden characters — `cat -v` shows nothing escaped).

### Key detail — the likely trigger

The failing edits targeted a region that had **only been inspected via bash (`grep`/`awk`), never via the `read` tool**, since the file's last modification. In the same session, content `replace` ops against regions that *had* just been read with `read` (E8/E9 edits at lines 235–240 and 326–330) succeeded on the first try. The tool hint itself says line-addressed ops are "immune to content drift" — and indeed `replace_lines` on the same line 744 succeeded immediately, without any intervening `read`.

**Hypothesis (a) — freshness gate with a wrong error message:** content-anchor ops require the target region to have been read via the `read` tool since the last change; bash inspection does not count. When the gate trips, the tool reports a *content-match failure* ("0/1 lines matched", "tried exact, trailing whitespace, and fuzzy matching") instead of stating the real reason ("region not read since last external modification"). 

**Hypothesis (b) — matcher bug on long multi-byte lines:** the target line was ~1.3 KB with multi-byte characters (`① ② — §`) before/around the anchor; byte/rune offset confusion could break line matching. Less likely given (a)'s correlation, but the earlier session also saw anchors containing U+2019 / `§` / `—` fail to match.

Either way this is a bug: at minimum the error is misleading; at worst the matcher is broken on valid input.

### Workaround

Use line-addressed ops (`replace_lines` with `start_line`/`end_line`), or `read` the exact region immediately before a content `replace`.

---

## Issue 2 — Mixed line-ops + content-ops in one batch corrupt the file

**Severity: High** — produces silently wrong file content.

### Actual behavior

A single batch containing both line-numbered edits and content-anchor edits was applied in order, but the line shifts produced by the line-ops were **not** accounted for when resolving the content anchors. Content edits then applied at stale positions, splicing text mid-paragraph: a paragraph on follow-up resolution landed inside an enumerated step, a defer-dialog sentence landed inside an unrelated bullet, and a `**Batch apply**` heading line was destroyed. Repair required whole-block `replace_lines` rewrites; one subsequent repair (`replace_lines 648–662`) itself consumed the final source line without re-adding it, dropping a sentence (caught later by reading the region).

### Expected

Either (a) the batch engine re-resolves content anchors against post-line-op coordinates, or (b) the tool **rejects** mixed batches with a clear error. Silently applying them against stale coordinates is the worst option.

### Workaround

Never mix op types in one batch. Line-ops first, verify, commit, then content ops in a separate batch.

---

## Issue 3 — Silent rollback: batch reports success, nothing persisted

**Severity: Critical** — false success report; violates the fundamental contract that a reported edit exists on disk. Directly caused two "applied cleanly" claims that were false.

### Actual behavior

Two separate edit batches returned normal success output (diff hunks shown), but the file on disk was unchanged. Detection only happened because of an independent check:

```bash
$ ls -la care-expert.md        # mtime still "Sep 25 09:32"
$ date                          # current time 10:39
$ grep -c "<marker>" care-expert.md   # 0 for every marker the batch claimed to add
```

No error was surfaced by the tool in either case.

### Expected

A reported success must be durable. If persistence fails, the tool must return an error — never a success diff.

### Workaround

After **every** batch: check `ls -la` mtime, grep for markers, and (once the file is tracked) `git diff --stat`. Treat the tool's returned diff view as *intent*, not *evidence* — verify by reading the actual file region.

---

## Issue 4 — Default `indent_mode` rewrites caller-supplied indentation

**Severity: Medium** — corrupts Markdown structure (nesting semantics are indentation-defined); each occurrence needed a repair edit.

### Actual behavior (3 reproductions this session)

1. **`insert_after` into a numbered-list guardrails item** — a paragraph supplied at column 0 was inserted with 3 leading spaces, nesting it under the list item.
2. **`insert_after` of a `> ⚠️` blockquote** — supplied at 3-space list-continuation indent; landed at 8 spaces (diff shows `+        > ⚠️`), and the preceding blank line gained trailing whitespace.
3. **`replace_lines` of a top-level `- **Connectivity**` bullet** — supplied at column 0, written with 2 leading spaces (`cat -A` shows `  - **Connectivity`), nesting it under the previous bullet. The `@@` diff hunk for the first repair attempt showed **no visible change** (normalize reproduced the same wrong indentation), making the no-op hard to spot.

All three were fixed only by re-applying with `indent_mode: "as-is"`.

### Expected

Caller-supplied content should be written **byte-for-byte by default**. Indentation normalization may be useful for code, but for a format where leading whitespace is semantic (Markdown lists, blockquotes, code fences) it must be opt-in, not the default. At minimum, `insert_after`/`replace_lines` with explicit `new_content` should never add indentation the caller did not write.

### Workaround

Always pass `indent_mode: "as-is"` for line-ops in Markdown (and verify with `cat -A` or `git diff`).

---

## Issue 5 — `replace_lines` rejects empty `new_content`

**Severity: Low** — API friction.

### Actual behavior

Attempting to replace a whitespace-only line with a truly empty line:

```
operation 'replace_lines' requires 'new_content' (or 'new_string') with the replacement text
Hint: ... To delete lines without replacement, use operation 'delete_lines'.
```

An empty string is valid replacement text (the goal was to *keep the line count*, not delete the line). The hint's alternative changes line numbering — a different operation.

### Expected

Accept `new_content: ""` (possibly with an explicit `allow_empty` flag if accidental empties are a concern).

---

## Issue 6 — `replace_pattern` replaced the whole line, dropping a prefix

**Severity: Medium.**

A `replace_pattern` op intended to substitute a matched fragment instead rewrote the entire line, dropping an unmatched prefix. Pattern ops appear to operate line-wise rather than match-wise. Workaround: prefer literal `replace` (after a fresh `read`) or line-ops.

## Issue 7 — `write` is not whole-file replace

**Severity: Medium.**

The `write` tool, given full-file content for an existing file, performed a partial/merge edit and displayed a diff that did not reflect the actual result. Callers must not rely on `write` to overwrite an existing large file deterministically; use `edit` line-ops instead.

---

## What works (keep)

- **Atomic batch failure is real**: the failing Issue-1 batch wrote 0 changes, matching the documented all-or-nothing semantics. This prevented partial corruption.
- **Line-addressed ops are reliable**: every `replace_lines`/`insert_after`/`delete_lines` op applied exactly where addressed (modulo Issue 4's indentation rewriting).
- The error hint pointing toward line-addressed ops for drifted regions is good advice — it should be the *documented primary mode* for large generated/edited files, not a fallback discovered by failure.

## Ruled out (initially suspected, not a bug)

- **`read` dedup ("unchanged since a previous read — content omitted")**: triggered right after a `replace_lines` repair attempt. Initially suspected as a stale cache. On review, that repair attempt had normalized to byte-identical content (a no-op), so the dedup was correct. Still, when verification is the goal, the omission forces a bash fallback (`sed -n` / `cat -A`); consider honoring an explicit re-read request.

## Recommended fixes (priority order)

1. **Issue 3**: success responses must be durable; surface persistence failures as errors. Add an internal post-write verification (size/mtime/hash) before returning success.
2. **Issue 1**: if a freshness gate exists, return an explicit, actionable error ("region not read since last change — call `read` first") instead of `not_found`; audit the matcher for byte/rune offset bugs on lines containing multi-byte characters.
3. **Issue 2**: reject mixed batches (line-ops + content-ops) with a clear error, or re-resolve anchors after line shifts.
4. **Issue 4**: make `indent_mode: as-is` the default for `insert_*`/`replace_lines`, or at least for non-code file types.
5. **Issue 5**: accept empty `new_content`.
6. **Issue 6/7**: clarify/repair line-wise semantics of `replace_pattern` and whole-file semantics of `write`.

## Reliable workaround playbook (field-tested this session)

1. Put the file under version control **first** (`git add` + baseline commit) so every edit is diffable and restorable.
2. One op type per batch; line-ops before content ops, never mixed.
3. Re-harvest line numbers with `grep -n` immediately before each batch — never reuse stale numbers.
4. Use `replace_lines`/`insert_after` with `indent_mode: "as-is"`; keep content anchors ASCII-only, short, and verified unique (`grep -c`) before use.
5. `read` the exact region immediately before any content `replace`.
6. After every batch: `git diff --stat` + read the actual region (not the tool's diff view) + check mtime. On corruption: `git restore` and redo in smaller pieces.
