#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=sentry-schedule-lib.sh
source "$DIR/sentry-schedule-lib.sh"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

# to_min: "HH:MM" → 자정 이후 분(정수). leading zero 안전해야 함.
check "to_min 14:47" "887" "$(to_min 14:47)"
check "to_min 05:30" "330" "$(to_min 05:30)"
check "to_min 00:00" "0"   "$(to_min 00:00)"
check "to_min 08:09 (leading zero)" "489" "$(to_min 08:09)"
check "to_min 23:59" "1439" "$(to_min 23:59)"

# sentry_should_fire <hhmm> <target>: 현재 시각이 목표 시각과 "정확히 같을 때만" fire.
sf() { sentry_should_fire "$@" && echo fire || echo no; }
check "fire: 정각 05:30==05:30" "fire" "$(sf 05:30 05:30)"
check "no: 05:31 != 05:30"       "no"   "$(sf 05:31 05:30)"
check "no: 05:29 != 05:30"       "no"   "$(sf 05:29 05:30)"
check "no: 14:47 != 05:30"       "no"   "$(sf 14:47 05:30)"
check "fire: 정각 22:16==22:16" "fire" "$(sf 22:16 22:16)"

exit "$fail"
