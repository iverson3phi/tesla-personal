#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "run 2 vent"   "afterblow 2 vent" "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 2 vent)"
check "run bare -> 3" "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh")"
check "run 99 -> 10"  "afterblow 10"     "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 99)"
check "run 3 only"    "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 3)"

exit "$fail"
