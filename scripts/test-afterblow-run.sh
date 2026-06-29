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
check "run 99 -> 3"   "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 99)"
check "run 3 only"    "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 3)"

# 중복 가드: 락이 이미 잡혀 있으면 새 트리거는 무시되고 PID파일도 안 남긴다.
# (테스트가 fd 9로 락을 선점 → afterblow-run.sh의 flock -n 8 실패. 9>&-로 자식에 fd9 미상속.)
TMP="$(mktemp -d)"
exec 9>"$TMP/lock"; flock -n 9 || { echo "FAIL - 테스트 락 셋업 실패"; exit 1; }
LOG="$TMP/out" AFTERBLOW_LOCK="$TMP/lock" AFTERBLOW_PIDFILE="$TMP/pid" AFTERBLOW_CMD=/bin/true \
	bash "$DIR/afterblow-run.sh" 1 9>&- >/dev/null 2>&1
check "락 점유 시 새 트리거 무시" "1" "$(grep -c '이미 진행 중' "$TMP/out")"
check "무시 시 PID파일 미생성"    "0" "$([ -f "$TMP/pid" ] && echo 1 || echo 0)"
exec 9>&- # 락 해제
LOG="$TMP/out2" AFTERBLOW_LOCK="$TMP/lock" AFTERBLOW_PIDFILE="$TMP/pid" AFTERBLOW_CMD=/bin/true \
	bash "$DIR/afterblow-run.sh" 1 >/dev/null 2>&1
check "락 해제 후 정상 실행" "1" "$(grep -c 'AFTERBLOW TRIGGERED' "$TMP/out2")"
rm -rf "$TMP"

exit "$fail"
