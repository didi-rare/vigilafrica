#!/usr/bin/env bash
# Boundary-load equivalence check (feature-continental-coverage tasks 3.7, 4.3).
#
# Take a snapshot of admin unit counts and every stored event's
# country_name/state_name before a boundary load, another after, and compare.
#
#   scripts/boundaries/equivalence.sh snapshot before.tsv
#   ... let the loader run (check the log for "boundary load committed") ...
#   scripts/boundaries/equivalence.sh snapshot after.tsv
#   scripts/boundaries/equivalence.sh compare before.tsv after.tsv
#
# The database is reached through $PSQL, a psql command line (default
# `psql "$DATABASE_URL"`). On a host where the database runs in Docker:
#
#   PSQL='docker exec -i <db-container> psql -U <user> -d <db>' \
#     scripts/boundaries/equivalence.sh snapshot before.tsv
#
# compare prints:
#   - unit counts per country and level, before and after, and every change;
#   - every event present in both snapshots whose country_name or state_name
#     changed (task 4.3 needs this list explained on the PR);
#   - how many events were added or removed between the snapshots (ingestion
#     keeps running on a live database; those are reported, not compared).
# It exits 1 when any unit count or any event label changed, 0 otherwise.
#
# For the first load (task 3.7, NG and GH only) the expected result is no
# change at all, with NG 37 and GH 16 ADM1 units on both sides.
#
# Read-only: snapshot runs two SELECTs and writes only the output file.

set -euo pipefail

PSQL="${PSQL:-psql \"\$DATABASE_URL\"}"

usage() {
  sed -n '2,30p' "$0" >&2
  exit 2
}

run_sql() {
  # SQL on stdin (works through `docker exec -i`). -X: ignore ~/.psqlrc;
  # -A -t: unaligned rows only; tab-separated fields.
  local tab=$'\t'
  printf '%s\n' "$1" | eval "$PSQL -X -q -A -t -v ON_ERROR_STOP=1 -F '$tab'"
}

snapshot() {
  local out="$1"
  {
    run_sql "SELECT 'unit', country_code, adm_level, count(*) FROM admin_boundaries GROUP BY 2, 3 ORDER BY 2, 3"
    run_sql "SELECT 'event', id, coalesce(country_name, '<null>'), coalesce(state_name, '<null>') FROM events ORDER BY id"
  } | tr -d '\r' >"$out"
  printf 'snapshot %s: %s unit rows, %s events\n' "$out" \
    "$(grep -c '^unit' "$out" || true)" "$(grep -c '^event' "$out" || true)"
}

compare() {
  local before="$1" after="$2"
  awk -F'\t' '
    FNR == 1 { side++ }
    $1 == "unit"  {
      key = $2 " ADM" $3
      if (side == 1) ub[key] = $4; else ua[key] = $4
      # No asorti (gawk only): keep first-seen order; snapshots are ORDER BY code, level.
      if (!(key in keys)) { keys[key] = 1; order[++nk] = key }
    }
    $1 == "event" { if (side == 1) eb[$2] = $3 "\t" $4; else ea[$2] = $3 "\t" $4 }
    END {
      changed = 0
      print "== admin unit counts (before -> after)"
      for (i = 1; i <= nk; i++) {
        k = order[i]
        b = (k in ub) ? ub[k] : 0; a = (k in ua) ? ua[k] : 0
        mark = (b == a) ? "" : "   <-- changed"
        if (b != a) changed++
        printf "  %-10s %6d -> %6d%s\n", k, b, a, mark
      }

      print "== events whose country or state changed (country | state, before -> after)"
      moved = 0; added = 0; removed = 0; same = 0
      for (id in eb) {
        if (!(id in ea)) { removed++; continue }
        if (eb[id] == ea[id]) { same++; continue }
        split(eb[id], x, "\t"); split(ea[id], y, "\t")
        printf "  %s  %s | %s  ->  %s | %s\n", id, x[1], x[2], y[1], y[2]
        moved++
      }
      for (id in ea) if (!(id in eb)) added++
      if (moved == 0) print "  (none)"
      printf "== %d events unchanged, %d changed, %d added since the first snapshot, %d removed\n", same, moved, added, removed
      changed += moved
      if (changed > 0) { print "RESULT: CHANGED"; exit 1 }
      print "RESULT: EQUIVALENT"
    }
  ' "$before" "$after"
}

case "${1:-}" in
  snapshot) [[ $# -eq 2 ]] || usage; snapshot "$2" ;;
  compare)  [[ $# -eq 3 ]] || usage; compare "$2" "$3" ;;
  *) usage ;;
esac
