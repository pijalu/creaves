#!/usr/bin/env bash
# clone_fixture_db.sh — build a throwaway review clone of the production
# database from an approved dump, for conversion E2E work (bugs.md
# BUG-50/BUG-51/BUG-52 rounds).
#
# Encodes the hard-won gotchas of the 2026-10-27 round:
#   * The clone DB is created with the SERVER DEFAULT charset/collation —
#     the dump is utf8mb4_0900_ai_ci (MySQL 8). NEVER force
#     utf8mb4_unicode_ci (or any explicit collation): FK creation and
#     joins during `pop migrate up` die with errors 1267/3780.
#   * The dump may carry DRIFT: schema objects from migrations that are
#     NOT recorded in schema_migration (e.g. outtaketypes.code). Those
#     migrations fail with duplicate-object errors; the script stamps
#     them into schema_migration and retries — the data already reflects
#     them. Stamp ONLY on failure, one version at a time.
#   * The dump's admin user has an unknown password hash — use
#     `buffalo task admin:reset-password` (grifts/create_admin.go).
#   * The app must be pointed at the clone via POP_PATH (pop reads
#     ./database.yml from $POP_PATH first), never by editing the tracked
#     database.yml.
#
# Usage: scripts/clone_fixture_db.sh <dump.sql.gz> <clone_db_name>
# Env:   MYSQL_USER/MYSQL_PASS (default creaves/creaves)
# After: POP_PATH=<cfg-dir> buffalo dev   (or run the built binary)
set -euo pipefail

DUMP=${1:?usage: clone_fixture_db.sh <dump.sql.gz> <clone_db_name>}
DBNAME=${2:?usage: clone_fixture_db.sh <dump.sql.gz> <clone_db_name>}
REPO=$(cd "$(dirname "$0")/.." && pwd)
MYSQL_USER=${MYSQL_USER:-creaves}
MYSQL_PASS=${MYSQL_PASS:-creaves}
CFG_DIR="/tmp/creaves-${DBNAME}-cfg"

m() { mysql -u"$MYSQL_USER" -p"$MYSQL_PASS" "$@" 2>/dev/null; }

echo "== dropping + creating $DBNAME (server default charset — no forced collation)"
m -e "DROP DATABASE IF EXISTS \`$DBNAME\`; CREATE DATABASE \`$DBNAME\`;"

echo "== loading dump"
case "$DUMP" in
  *.gz) gzip -dc "$DUMP" | m "$DBNAME" ;;
  *)    m "$DBNAME" < "$DUMP" ;;
esac

echo "== writing POP_PATH config BEFORE migrating (pop must NEVER fall back to the project database.yml): $CFG_DIR"
mkdir -p "$CFG_DIR"
cat > "$CFG_DIR/database.yml" <<YAML
# E2E fixture override (POP_PATH wins over project database.yml):
# "$DBNAME" is a THROWAWAY REVIEW CLONE — never point this at prod.
development:
  dialect: "mysql"
  url: "mysql://$MYSQL_USER:$MYSQL_PASS@(localhost:3306)/$DBNAME?parseTime=true&multiStatements=true&readTimeout=60s"
  pool: 25
  idlepool: 5
  connmaxlifetime: 10m
  connmaxidletime: 1m

test:
  dialect: "mysql"
YAML

echo "== migrating (stamping drifted-but-applied versions on failure)"
for i in $(seq 1 40); do
  OUT=$(cd "$REPO" && POP_PATH="$CFG_DIR" buffalo pop migrate up 2>&1 | grep -v '^\[POP\]' | grep -v mysqldump || true)
  V=$(printf '%s' "$OUT" | grep -o 'migrations/[0-9]*_' | head -1 | tr -dc 0-9 || true)
  if [ -z "$V" ]; then
    echo "   migrations complete (iter $i)"
    break
  fi
  ERR=$(printf '%s' "$OUT" | grep -o 'Error [0-9]* ([^)]*)' | head -1)
  echo "   stamping drifted $V ($ERR)"
  m "$DBNAME" -e "INSERT INTO schema_migration(version) VALUES ('$V');"
done

echo "== resetting admin/admin (fixture only — NEVER prod)"
(cd "$REPO" && POP_PATH="$CFG_DIR" buffalo task admin:reset-password 2>&1 | grep -v '^\[POP\]' || true)

echo "== next steps"
cat <<NEXT
  export POP_PATH=$CFG_DIR
  buffalo dev                        # boots on :3000-dev-port against $DBNAME
  PORT=3002 buffalo dev              # or another port to keep prod-ish flows free
  buffalo task careplan:validate-feeding    # must print VALIDATION: PASS
  buffalo task careplan:validate-treatments # must print VALIDATION: PASS
NEXT
