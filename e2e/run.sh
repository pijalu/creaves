#!/usr/bin/env bash
# E2E validation suite (plan §7.2) — agent-browser driven, both apps, 4 languages.
#
# Prereqs: e2e environment up per .agents/skills/e2e-testing/SKILL.md
#   - creaves on :3000 seeded (db:seed + db:seed:e2e), console on :3001 seeded
#   - instance A rows present in console (resync done — this suite re-does it
#     in the contract section anyway)
#
# Usage: ./e2e/run.sh [--contract-only]
# Every assertion traces to e2e/EXPECTATIONS.md. Exit 0 = all green.

set -u
cd "$(dirname "$0")/.."
ROOT="$(pwd)"

CREAVES=http://127.0.0.1:3000
CONSOLE=http://127.0.0.1:3001
MYSQL="mysql -ucreaves -pcreaves"

PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  ok   $1"; }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL $1"; }
check(){ # check <label> <haystack> <needle>
  if printf '%s' "$2" | grep -qF -- "$3"; then ok "$1"; else bad "$1 — missing: $3"; fi
}
check_count(){ # check_count <label> <expected> <actual>
  if [ "$2" = "$3" ]; then ok "$1 ($3)"; else bad "$1 — want $2 got $3"; fi
}
section(){ echo; echo "== $1"; }

AB="agent-browser"
page(){ $AB open "$1" >/dev/null && $AB wait --load networkidle >/dev/null && $AB get text body; }

# session cookie for curl CSV assertions (current page's app)
cookie_for(){ # cookie_for <session-cookie-name>
  $AB cookies get --json | python3 -c "import sys,json;d=json.load(sys.stdin);print(next((c['value'] for c in d['data']['cookies'] if c['name']=='$1'),''))"
}

# ref_of <kind-pattern> <snapshot-text> -> first matching element ref.
# Snapshot lines look like: - textbox "Species" [required, ref=e4]
# (attributes may precede ref=, so never anchor on "[ref=").
ref_of(){ # ref_of <label-regex> <snapshot> [nth]
  printf '%s\n' "$2" | sed -n "s/.*$1[^]]*ref=\(e[0-9]*\).*/\1/p" | head -"${3:-1}" | tail -1
}

section "0. sanity: both apps reachable"
code_a=$(curl -s -o /dev/null -w '%{http_code}' "$CREAVES/")
code_b=$(curl -s -o /dev/null -w '%{http_code}' "$CONSOLE/")
check_count "creaves up (302)" 302 "$code_a"
check_count "console up (302)" 302 "$code_b"
[ "$FAIL" -gt 0 ] && { echo "apps not running, aborting"; exit 1; }

############################################################
section "Creaves: login admin/admin"
page "$CREAVES/auth/new" >/dev/null
$AB snapshot -i | grep -q 'textbox "Login"' || { echo "login form not found"; exit 1; }
LSNAP=$($AB snapshot -i)
L=$(ref_of 'textbox "Login"' "$LSNAP")
P=$(ref_of 'textbox "Password"' "$LSNAP")
B=$(ref_of 'button "Sign In!"' "$LSNAP")
$AB fill "@$L" admin >/dev/null; $AB fill "@$P" admin >/dev/null; $AB click "@$B" >/dev/null
$AB wait --load networkidle >/dev/null
[ "$($AB get url)" = "$CREAVES/" ] && ok "logged in" || bad "login redirect — $($AB get url)"

############################################################
section "Creaves 0.5 reset non-fixture state + resync instance A"
# Repeatability: previous runs leave a UI-created animal behind (contract
# section). Wipe everything that is not part of the fixed fixture set, then
# re-run a resync so the console holds exactly the 9 fixture rows for A.
FIXTURE_IDS="240008,240009,250001,250002,250003,250004,250005,250006,250007"
$MYSQL creaves 2>/dev/null <<SQL
DELETE FROM animals WHERE id NOT IN ($FIXTURE_IDS);
DELETE FROM outtakes WHERE id NOT IN (SELECT outtake_id FROM animals WHERE outtake_id IS NOT NULL);
DELETE FROM discoveries WHERE id NOT IN (SELECT discovery_id FROM animals WHERE discovery_id IS NOT NULL);
DELETE FROM intakes WHERE id NOT IN (SELECT intake_id FROM animals WHERE intake_id IS NOT NULL);
DELETE FROM event_streams WHERE instance_id='e2e-instance-a';
UPDATE resync_runs SET status='failed', finished_at=NOW(), errors='reset by e2e' WHERE status='running';
SQL
# NOTE: console event_streams for A are KEPT on purpose: the resync
# redelivers the same deterministic event UUIDs and the console must rebuild
# the wiped rows through its disaster-recovery path (redelivery of an already
# processed event whose consolidated row is missing re-applies it).
$MYSQL consolidation -e "DELETE FROM consolidated_animals WHERE instance_id='e2e-instance-a'" 2>/dev/null
page "$CREAVES/webhook_resync" >/dev/null
RB=$(ref_of 'button "Start resync"' "$($AB snapshot -i)")
if [ -n "$RB" ]; then
  $AB click "@$RB" >/dev/null
  for i in $(seq 1 30); do
    sleep 2
    st=$($MYSQL creaves -sN -e "SELECT status FROM resync_runs ORDER BY created_at DESC LIMIT 1" 2>/dev/null)
    [ "$st" = "completed" ] && break
  done
  check_count "reset resync completed" completed "$st"
  # wait until the pusher has delivered all rows to the console (5s tick)
  for i in $(seq 1 20); do
    n=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a'" 2>/dev/null)
    [ "$n" = "9" ] && break
    sleep 3
  done
else
  bad "reset: Start resync button not found"
fi
n=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a'" 2>/dev/null)
check_count "console A reset to 9 fixture rows" 9 "$n"

############################################################
section "Creaves 1. /animals search filters (row counts)"
rows(){ # rows <query> -> row count of results table
  page "$CREAVES/animals/?$1" >/dev/null
  $AB get count "table tbody tr"
}
check_count "no filter"        9 "$(rows '')"
check_count "year=2025"        7 "$(rows 'year=2025')"
check_count "year=2024"        2 "$(rows 'year=2024')"
TID_A=$($MYSQL creaves -sN -e "SELECT id FROM animaltypes WHERE name='E2E_TA'" 2>/dev/null)
AGE_ADULT=$($MYSQL creaves -sN -e "SELECT id FROM animalages WHERE name='E2E Adult'" 2>/dev/null)
EC2=$($MYSQL creaves -sN -e "SELECT id FROM entry_causes WHERE id='E2E_EC2'" 2>/dev/null)
OT_REL=$($MYSQL creaves -sN -e "SELECT id FROM outtaketypes WHERE name='E2E_REL'" 2>/dev/null)
OT_DCD=$($MYSQL creaves -sN -e "SELECT id FROM outtaketypes WHERE name='E2E_DCD'" 2>/dev/null)
OT_ERR=$($MYSQL creaves -sN -e "SELECT id FROM outtaketypes WHERE name='E2E_ERR'" 2>/dev/null)
check_count "type=E2E_TA"      4 "$(rows "animaltype_id=$TID_A")"
check_count "species=E2E_Newt" 2 "$(rows 'species=E2E_Newt')"
check_count "entry cause EC2"  4 "$(rows "entry_cause_id=$EC2")"
check_count "age adult"        4 "$(rows "animalage_id=$AGE_ADULT")"
check_count "ring~RING-00"     8 "$(rows 'ring=RING-00')"  # 6x 2025 + 240008/240009 (250006 NULL)
check_count "ring exact 001"   1 "$(rows 'ring=E2E-RING-001')"
check_count "outtake REL"      3 "$(rows "outtaketype_id=$OT_REL")"
check_count "outtake DCD"      1 "$(rows "outtaketype_id=$OT_DCD")"
check_count "outtake ERR excluded" 0 "$(rows "outtaketype_id=$OT_ERR")"
check_count "combo 2025+EC1+juv" 4 "$(rows "year=2025&entry_cause_id=E2E_EC1&animalage_id=aaaaaaaa-0000-0000-0000-0000000000e1")"

############################################################
section "Creaves 2. search CSV export content"
CK=$(cookie_for _creaves_session)
csv=$(curl -s -b "_creaves_session=$CK" "$CREAVES/animals/search/export.csv?year=2025")
n=$(printf '%s' "$csv" | tail -n +2 | grep -c .)
check_count "csv 2025 data rows" 7 "$n"
# search CSV has no city column and does not exclude error-outtake animals;
# the ';'+quote escaping fixture is exercised on the CONSOLE export instead.
check "csv exact DCD row" "$csv" 'E2E-RING-002;2025/06/15 10:00;E2E_C1;E2E_D1;2025/06/15 10:00;E2E_DCD;;'
check "csv contains ring 001" "$csv" 'E2E-RING-001'
ring5=$(printf '%s' "$csv" | grep -c 'E2E-RING-005' || true)
check_count "csv includes error animal (RING-005)" 1 "$ring5"
hdr=$(printf '%s' "$csv" | head -1)
check "csv header non-empty" "$hdr" ";"

############################################################
section "Creaves 3. /reports/annual exact numbers"
body25=$(page "$CREAVES/reports/annual?year=2025")
for pair in \
  "E2E_Hedgehog	2	33.3%" "E2E_Sparrow	2	33.3%" "E2E_Newt	1	16.7%" "Unknown	1	16.7%" "Total	6	100.0%" \
  "E2E_Aves	3	50.0%" "E2E_Mammalia	2	33.3%" "E2E Group A	3	50.0%" "Unknown	3	50.0%" \
  "E2E Native	3	50.0%" "E2E Adult	3	50.0%" "E2E Juvenile	3	50.0%" \
  "E2E_REL	2	66.7%" "E2E_DCD	1	33.3%" "Total	3	100.0%" \
  "Dead	1	33.3%" "Alive	2	66.7%" "Released	2	66.7%" \
  "E2E_C1	3	50.0%" "E2E_C2	3	50.0%" "E2E_N1 / E2E_C1 / E2E_D1	3	50.0%" "E2E_N1 / E2E_C2	3	50.0%" "E2E_N1	6	100.0%" \
; do check "2025: $pair" "$body25" "$(printf '%b' "$pair")"; done
nerr=$(printf '%s' "$body25" | grep -c 'E2E_ERR' || true)
check_count "2025: E2E_ERR never shown" 0 "$nerr"

body24=$(page "$CREAVES/reports/annual?year=2024")
for pair in \
  "E2E_Hedgehog	1	50.0%" "E2E_Sparrow	1	50.0%" "Total	2	100.0%" \
  "E2E_REL	1	100.0%" "Alive	1	100.0%" "Released	1	100.0%" "Total	1	100.0%" \
  "E2E_N1	2	100.0%" \
; do check "2024: $pair" "$body24" "$(printf '%b' "$pair")"; done

############################################################
section "Creaves 4. annual CSV export"
acsv=$(curl -s -b "_creaves_session=$CK" "$CREAVES/reports/annual/export.csv?year=2025")
check "annual csv species section" "$acsv" "E2E_Hedgehog"
check "annual csv total 6" "$acsv" "6"
check "annual csv outtake REL" "$acsv" "E2E_REL"
nsec=$(printf '%s' "$acsv" | grep -c '^' )
[ "$nsec" -gt 24 ] && ok "annual csv multi-section ($nsec lines)" || bad "annual csv too short ($nsec lines)"

############################################################
section "Creaves 5. 4-language spot checks (same numbers, localized labels)"
title_for(){ case "$1" in fr) echo "Statistiques annuelles";; en-US) echo "Annual statistics";; de) echo "Jahresstatistik";; nl) echo "Jaarlijkse statistieken";; esac; }
for lang in fr en-US de nl; do
  page "$CREAVES/lang/?lang=$lang&url=/reports/annual?year=2025" >/dev/null
  b=$($AB get text body)
  check "$lang title" "$b" "$(title_for "$lang")"
  check "$lang numbers stable" "$b" "$(printf 'E2E_Hedgehog\t2\t33.3%%')"
done
page "$CREAVES/lang/?lang=en-US&url=/" >/dev/null   # reset to en

############################################################
section "Creaves 6. nav links (Reports menu → annual, 4 variants)"
for lang in fr en-US de nl; do
  page "$CREAVES/lang/?lang=$lang&url=/" >/dev/null
  html=$($AB get html "body")
  check "$lang nav has /reports/annual" "$html" "/reports/annual"
done
page "$CREAVES/lang/?lang=en-US&url=/" >/dev/null   # reset to en before console/wizard sections

############################################################
section "Console: login admin/admin123"
page "$CONSOLE/auth/new" >/dev/null
CS=$($AB snapshot -i)
L=$(ref_of 'textbox "Login"' "$CS")
P=$(ref_of 'textbox "Password"' "$CS")
BTN=$(ref_of 'button "Login"' "$CS")
if [ -z "$L" ] || [ -z "$P" ] || [ -z "$BTN" ]; then
  bad "console login form not found (L=$L P=$P BTN=$BTN)"
  printf '%s\n' "$CS" | head -20
else
  $AB fill "@$L" admin >/dev/null; $AB fill "@$P" admin123 >/dev/null; $AB click "@$BTN" >/dev/null
  $AB wait --load networkidle >/dev/null
  $AB get url | grep -q "auth" && bad "console login — still on auth" || ok "console logged in ($($AB get url))"
fi

############################################################
section "Console 1. /consolidated_animals filters"
crows(){ page "$CONSOLE/consolidated_animals/?$1" >/dev/null; $AB get count "table tbody tr"; }
check_count "console no filter" 15 "$(crows '')"
check_count "console scope A" 9 "$(crows 'instance_id=e2e-instance-a')"
check_count "console scope B" 6 "$(crows 'instance_id=e2e-instance-b')"
check_count "console ring~RING-001" 2 "$(crows 'ring=RING-001')"
check_count "console age E2E Adult" 4 "$(crows 'animal_age=E2E Adult')"
check_count "console outtake E2E_REL" 3 "$(crows 'outtake_type=E2E_REL')"
check_count "console outtake B REL" 2 "$(crows 'instance_id=e2e-instance-b&outtake_type=E2EB_REL')"
check_count "console entry cause A C1" 5 "$(crows 'instance_id=e2e-instance-a&entry_cause=E2E_C1+%E2%87%A8+E2E_D1')"
check_count "console entry cause B C2" 2 "$(crows 'instance_id=e2e-instance-b&entry_cause=E2EB_C2')"

############################################################
section "Console 2. export CSV with filters+scope"
CCK=$(cookie_for _creaves_console_session)
[ -z "$CCK" ] && CCK=$(cookie_for _creaves_console_session)
ccsv=$(curl -s -b "_creaves_console_session=$CCK" "$CONSOLE/consolidated_animals/export.csv?instance_id=e2e-instance-b")
check "console csv has B city escaped" "$ccsv" 'E2EB City; ""Sud""'
na=$(printf '%s' "$ccsv" | grep -c 'E2E-RING' || true)
check_count "console csv B-only (no E2E-RING)" 0 "$na"

############################################################
section "Console 3. /reports/annual scopes exact numbers (2025)"
cbody_all=$(page "$CONSOLE/reports/annual?year=2025")
for pair in \
  "E2E_Hedgehog	2" "E2EB_Fox	2" "E2EB_Owl	2" "Unknown	2" "Total	12" \
  "E2E_Aves	4" "E2EB_Aves	2" "E2E_Mammalia	2" "E2EB_Mammalia	2" "Unknown	2" \
  "E2E Adult	3" "E2EB Adult	2" "E2E Juvenile	4" "E2EB Juvenile	2" "Unknown	1" \
  "E2E_REL	2" "E2EB_REL	1" "E2E_DCD	1" "E2EB_DCD	1" "E2E_ERR	1" "Total	6" \
  "E2E_N1	7" "E2EB_N1	2" "E2EB_N2	2" \
; do check "all 2025: $pair" "$cbody_all" "$(printf '%b' "$pair")"; done

cbody_a=$(page "$CONSOLE/reports/annual?year=2025&instance_id=e2e-instance-a")
check "A 2025 total 7" "$cbody_a" "$(printf 'Total\t7')"
check "A 2025 outtake total 4" "$cbody_a" "$(printf 'Total\t4')"
check "A 2025 ERR included (no console error exclusion)" "$cbody_a" "E2E_ERR"
nab=$(printf '%s' "$cbody_a" | grep -c 'E2EB' || true)
check_count "A 2025 no B rows" 0 "$nab"

cbody_b=$(page "$CONSOLE/reports/annual?year=2025&instance_id=e2e-instance-b")
check "B 2025 total 5" "$cbody_b" "$(printf 'Total\t5')"
check "B 2025 outtake total 2" "$cbody_b" "$(printf 'Total\t2')"
check "B 2025 Fox" "$cbody_b" "E2EB_Fox"
nba=$(printf '%s' "$cbody_b" | grep -c 'E2E_Hedgehog' || true)
check_count "B 2025 no A rows" 0 "$nba"

cbody_24=$(page "$CONSOLE/reports/annual?year=2024")
check "all 2024 total 3 (A2+B1)" "$cbody_24" "$(printf 'Total\t3')"
check "all 2024 outtake total 2" "$cbody_24" "$(printf 'Total\t2')"

############################################################
section "Console 4. annual CSV per scope"
acsv_all=$(curl -s -b "_creaves_console_session=$CCK" "$CONSOLE/reports/annual/export.csv?year=2025")
check "console annual csv all: ERR row" "$acsv_all" "E2E_ERR"
acsv_b=$(curl -s -b "_creaves_console_session=$CCK" "$CONSOLE/reports/annual/export.csv?year=2025&instance_id=e2e-instance-b")
check "console annual csv B: Fox" "$acsv_b" "E2EB_Fox"
nerrb=$(printf '%s' "$acsv_b" | grep -c 'E2E_ERR' || true)
check_count "console annual csv B: no A ERR" 0 "$nerrb"

############################################################
section "Console 5. 4 languages + navbar Reports menu"
for lang in fr en-US de nl; do
  page "$CONSOLE/lang/?lang=$lang&url=/reports/annual?year=2025" >/dev/null
  b=$($AB get text body)
  h=$($AB get html body)
  check "console $lang title" "$b" "$(title_for "$lang")"
  check "console $lang numbers stable" "$b" "E2EB_Fox"
  check "console $lang navbar reports menu" "$h" "reportsDropdown"
done
page "$CONSOLE/lang/?lang=en-US&url=/" >/dev/null

############################################################
if [ "${1:-}" != "--skip-contract" ]; then
section "Contract e2e. create animal in Creaves UI → console row"
# Reception wizard: step0 (count) → step1 (type/species/age) → step2
# (discovery: entry cause) → step3 (intake) → Finish → /animals/<id>.
# Select widgets are select2-style: set value via JS + change event.
page "$CREAVES/reception/new" >/dev/null
S0=$($AB snapshot -i)
NEXT0=$(ref_of 'button "Next' "$S0")
$AB click "@$NEXT0" >/dev/null; sleep 1
S1=$($AB snapshot -i)
TSPECIES=$(ref_of 'textbox "Species' "$S1")
[ -n "$TSPECIES" ] && $AB fill "@$TSPECIES" "E2E_Hedgehog" >/dev/null
$AB eval 'var s=document.querySelector("select[name*=\"Animaltype\"]"); if(s){s.value="bbbbbbbb-0000-0000-0000-0000000000e1"; s.dispatchEvent(new Event("change",{bubbles:true}))}; var a=document.querySelector("select[name*=\"Animalage\"]"); if(a){a.value="aaaaaaaa-0000-0000-0000-0000000000e1"; a.dispatchEvent(new Event("change",{bubbles:true}))}; "ok"' >/dev/null
NEXT=$(ref_of 'button "Next' "$S1")
$AB click "@$NEXT" >/dev/null; sleep 1
$AB eval 'var s=document.querySelector("select[name=\"Discovery.EntryCauseID\"]")||[...document.querySelectorAll("select")].find(x=>[...x.options].some(o=>o.value==="E2E_EC1")); if(s){s.value="E2E_EC1"; s.dispatchEvent(new Event("change",{bubbles:true})); "set"} else "noselect"' >/dev/null
S2=$($AB snapshot -i)
NEXT2=$(ref_of 'button "Next' "$S2")
$AB click "@$NEXT2" >/dev/null; sleep 1
S3=$($AB snapshot -i)
FIN=$(ref_of 'button "Finish' "$S3")
$AB click "@$FIN" >/dev/null; sleep 2
NEWURL=$($AB get url)
NEWID=$(printf '%s' "$NEWURL" | sed -n 's|.*/animals/\([0-9]*\).*|\1|p')
[ -n "$NEWID" ] && ok "animal created id=$NEWID" || bad "wizard did not redirect to /animals/<id> ($NEWURL)"
if [ -n "$NEWID" ]; then
  sleep 8  # pusher tick
  crow=$($MYSQL consolidation -sN -e "SELECT CONCAT(IFNULL(animal_age,'NULL'),'|',IFNULL(animal_type,'NULL'),'|',IFNULL(entry_cause,'NULL')) FROM consolidated_animals WHERE animal_id=$NEWID" 2>/dev/null)
  check "console row has v2 fields" "$crow" "E2E Juvenile|E2E_TA|E2E_C1"
fi
fi

############################################################
section "Contract e2e. resync backfill after wiping instance A"
# Remove the animal created by the wizard section above so the resync restores
# exactly the 9 fixture rows.
$MYSQL creaves 2>/dev/null <<SQL
DELETE FROM animals WHERE id NOT IN ($FIXTURE_IDS);
DELETE FROM outtakes WHERE id NOT IN (SELECT outtake_id FROM animals WHERE outtake_id IS NOT NULL);
DELETE FROM discoveries WHERE id NOT IN (SELECT discovery_id FROM animals WHERE discovery_id IS NOT NULL);
DELETE FROM intakes WHERE id NOT IN (SELECT intake_id FROM animals WHERE intake_id IS NOT NULL);
SQL
$MYSQL consolidation -e "DELETE FROM consolidated_animals WHERE instance_id='e2e-instance-a'; DELETE FROM event_streams WHERE instance_id='e2e-instance-a';" 2>/dev/null
$MYSQL creaves -e "DELETE FROM event_streams WHERE instance_id='e2e-instance-a';" 2>/dev/null
page "$CREAVES/webhook_resync" >/dev/null
RB=$(ref_of 'button "Start resync"' "$($AB snapshot -i)")
$AB click "@$RB" >/dev/null
for i in $(seq 1 30); do
  sleep 2
  st=$($MYSQL creaves -sN -e "SELECT status FROM resync_runs ORDER BY created_at DESC LIMIT 1" 2>/dev/null)
  [ "$st" = "completed" ] && break
done
[ "$st" = "completed" ] && ok "resync run completed" || bad "resync run status: $st"
# wait until the pusher has delivered all rows (5s tick)
for i in $(seq 1 20); do
  n=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a'" 2>/dev/null)
  [ "$n" = "9" ] && break
  sleep 3
done
n=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a'" 2>/dev/null)
check_count "resync restored 9 rows" 9 "$n"
nfull=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a' AND animal_age IS NOT NULL AND entry_cause IS NOT NULL" 2>/dev/null)
check_count "backfilled rows have v2 columns" 9 "$nfull"
nota=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a' AND outtake_type IS NOT NULL" 2>/dev/null)
check_count "outtake types backfilled (REL3+DCD1+ERR1)" 5 "$nota"
tr=$($MYSQL consolidation -sN -e "SELECT COUNT(*) FROM consolidated_animals WHERE instance_id='e2e-instance-a' AND translations IS NOT NULL" 2>/dev/null)
[ "$tr" -ge 1 ] && ok "translations delivered on $tr rows" || bad "translations missing after resync"

############################################################
echo
echo "=================================="
echo "PASS=$PASS FAIL=$FAIL"
[ "$FAIL" = 0 ] && echo "E2E SUITE GREEN" || echo "E2E SUITE RED"
exit "$FAIL"
