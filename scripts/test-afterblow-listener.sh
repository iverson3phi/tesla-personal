#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LOG=/dev/null
# shellcheck source=afterblow-listener.sh
source "$DIR/afterblow-listener.sh"

RECORD="$(mktemp)"
record() { printf '%s\n' "$*" >>"$RECORD"; }

printf 'afterblow 2 vent\n\nhello world\nafterblow 1\n' | ab_consume_stream record

mapfile -t lines <"$RECORD"
rm -f "$RECORD"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "count" "2"      "${#lines[@]}"
check "first" "2 vent" "${lines[0]:-}"
check "second" "1"     "${lines[1]:-}"

exit "$fail"
