#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=afterblow-lib.sh
source "$DIR/afterblow-lib.sh"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "sanitize 2"         "2"      "$(ab_sanitize_minutes 2)"
check "sanitize junk -> 3" "3"      "$(ab_sanitize_minutes abc)"
check "sanitize 0 -> 1"    "1"      "$(ab_sanitize_minutes 0)"
check "sanitize 99 -> 10"  "10"     "$(ab_sanitize_minutes 99)"

check "bare afterblow -> 3" "3"      "$(ab_parse_message 'afterblow')"
check "afterblow 2 -> 2"    "2"      "$(ab_parse_message 'afterblow 2')"
check "afterblow 3 vent"    "3 vent" "$(ab_parse_message 'afterblow 3 vent')"
check "afterblow 99 -> 10"  "10"     "$(ab_parse_message 'afterblow 99')"
check "afterblow abc -> 3"  "3"      "$(ab_parse_message 'afterblow abc')"

if ab_parse_message 'hello world' >/dev/null 2>&1; then
	printf 'FAIL - non-afterblow line ignored\n'; fail=1
else
	printf 'ok   - non-afterblow line ignored\n'
fi

got="$(ab_dispatch_line 'afterblow 4 vent' echo)"
check "dispatch args" "4 vent" "$got"

exit "$fail"
