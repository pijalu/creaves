#!/usr/bin/env bash
# Quality gate (bugs.md guideline #7 + care-plan round-2 gate policy).
#
# Runs each check SEPARATELY (guideline: never chain the linters with &&):
#   - go vet, staticcheck, go test  → fatal on failure
#   - gocognit -over 15 / gocyclo -over 12 → "no NEW offenders" policy:
#     both tools exit 1 whenever ANY function exceeds the threshold, and
#     this tree carries a fixed set of pre-existing offenders (110/97 at
#     the round-2 baseline). The gate therefore diffs the offender list
#     against the committed baseline and fails only on NEW entries.
#
# Exit 0 = pass. Regenerate the baselines ONLY deliberately, when a
# refactor legitimately changes the offender set:
#   gocognit -over 15 . | awk '{$1="";print}' | awk '{print $1,$2}' | sort \
#     > scripts/quality-gate-baseline/gocognit.txt
#   gocyclo -over 12 . | awk '{$1="";print}' | awk '{print $1,$2}' | sort \
#     > scripts/quality-gate-baseline/gocyclo.txt
set -uo pipefail
cd "$(dirname "$0")/.."

fail=0

echo "== go vet"
go vet ./... || fail=1

echo "== staticcheck"
staticcheck ./... || fail=1

echo "== go test"
go test -count=1 ./actions ./models || fail=1

base=scripts/quality-gate-baseline
check_list() { # <tool> <over> <baseline-file>
  local tool=$1 over=$2 basefile=$3 out new
  out=$(mktemp)
  "$tool" -over "$over" . >"$out" 2>&1
  new=$(awk '{$1="";print}' "$out" | awk '{print $1,$2}' | sort | comm -13 "$basefile" -)
  if [ -n "$new" ]; then
    echo "== NEW $tool offenders (gate FAIL):"
    echo "$new"
    fail=1
  else
    echo "== $tool: no new offenders (baseline $(wc -l <"$basefile" | tr -d ' ') functions)"
  fi
  rm -f "$out"
}

check_list gocognit 15 "$base/gocognit.txt"
check_list gocyclo 12 "$base/gocyclo.txt"

if [ "$fail" -ne 0 ]; then
  echo "QUALITY GATE: FAIL"
else
  echo "QUALITY GATE: OK"
fi
exit "$fail"
