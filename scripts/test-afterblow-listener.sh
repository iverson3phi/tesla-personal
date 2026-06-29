#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LOG=/dev/null
# shellcheck source=afterblow-listener.sh
source "$DIR/afterblow-listener.sh"

RECORD="$(mktemp)"
record() { printf '%s\n' "$*" >>"$RECORD"; }
# 실제 바이너리 호출을 막고 분기 결과만 기록한다.
run_sentry() { printf 'sentry %s\n' "$1" >>"$RECORD"; }
run_cancel() { printf 'cancel\n' >>"$RECORD"; }

printf 'afterblow 2 vent\n\nhello world\nafterblow cancel\nsentry on\nsentry status\nafterblow 1\n' \
	| ab_consume_stream record

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

check "count"  "5"              "${#lines[@]}"
check "line0"  "2 vent"         "${lines[0]:-}"
check "line1"  "cancel"         "${lines[1]:-}"
check "line2"  "sentry on"      "${lines[2]:-}"
check "line3"  "sentry status"  "${lines[3]:-}"
check "line4"  "1"              "${lines[4]:-}"

exit "$fail"
