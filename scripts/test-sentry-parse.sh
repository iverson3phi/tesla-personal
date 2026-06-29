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

check "sentry on"     "on"     "$(ab_parse_sentry 'sentry on')"
check "sentry off"    "off"    "$(ab_parse_sentry 'sentry off')"
check "sentry status" "status" "$(ab_parse_sentry 'sentry status')"

# 비대상/불량 입력은 return 1 (출력 없음)
for bad in 'sentry maybe' 'sentry' 'afterblow 2' 'hello' 'sentry on extra'; do
	if ab_parse_sentry "$bad" >/dev/null 2>&1; then
		printf 'FAIL - rejected: %s\n' "$bad"; fail=1
	else
		printf 'ok   - rejected: %s\n' "$bad"
	fi
done

exit "$fail"
