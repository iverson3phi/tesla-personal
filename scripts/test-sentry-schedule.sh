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

# sentry_action <hhmm> <슬롯...>: 슬롯 합집합의 경계 분에만 on/off.
a() { sentry_action "$@"; }
one=(05:30-22:16)
check "1세트: 05:30 on"        "on"  "$(a 05:30 "${one[@]}")"
check "1세트: 05:29 없음"      ""    "$(a 05:29 "${one[@]}")"
check "1세트: 05:31 없음(정시)" ""    "$(a 05:31 "${one[@]}")"
check "1세트: 22:16 off"       "off" "$(a 22:16 "${one[@]}")"
check "1세트: 14:47 없음"      ""    "$(a 14:47 "${one[@]}")"

two=(05:30-08:00 18:00-23:00)
check "2세트: 05:30 on"  "on"  "$(a 05:30 "${two[@]}")"
check "2세트: 08:00 off" "off" "$(a 08:00 "${two[@]}")"
check "2세트: 18:00 on"  "on"  "$(a 18:00 "${two[@]}")"
check "2세트: 23:00 off" "off" "$(a 23:00 "${two[@]}")"
check "2세트: 12:00 없음" ""   "$(a 12:00 "${two[@]}")"

overlap=(08:00-12:00 10:00-14:00)
check "겹침: 08:00 on"   "on"  "$(a 08:00 "${overlap[@]}")"
check "겹침: 10:00 없음" ""    "$(a 10:00 "${overlap[@]}")"
check "겹침: 12:00 없음" ""    "$(a 12:00 "${overlap[@]}")"
check "겹침: 14:00 off"  "off" "$(a 14:00 "${overlap[@]}")"

check "맞닿음: 12:00 없음" "" "$(a 12:00 08:00-12:00 12:00-14:00)"

check "자정넘김: 22:00 on"   "on"  "$(a 22:00 22:00-06:00)"
check "자정넘김: 00:00 없음" ""    "$(a 00:00 22:00-06:00)"
check "자정넘김: 06:00 off"  "off" "$(a 06:00 22:00-06:00)"
check "00:00 on 경계"        "on"  "$(a 00:00 00:00-01:00)"
check "슬롯 없음"            ""    "$(a 05:30)"

exit "$fail"
