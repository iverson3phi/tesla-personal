#!/usr/bin/env bash
# afterblow 트리거 핸들러.
# - 인자로 받은 분/환기로 tesla-sentry afterblow 를 실행한다.
# - 디바운스: 짧은 시간 안의 중복 트리거는 무시한다(실수 더블탭 대비).
# - AFTERBLOW_DRY_RUN=1 이면 실제 명령 대신 해석된 인자를 출력하고 종료(테스트용).
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
# shellcheck source=afterblow-lib.sh
source "$DIR/afterblow-lib.sh"

# ── 설정 (환경변수로 덮어쓸 수 있음) ──────────────────────────────
LOG="${LOG:-$ROOT/tesla.log}"
# ──────────────────────────────────────────────────────────────────

log() { printf '%s [run] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

# 입력 해석(공개 토픽 → 신뢰 불가 입력이므로 재검증).
minutes="$(ab_sanitize_minutes "${1:-}")"
vent=""
for a in "$@"; do [ "$a" = "vent" ] && vent="vent"; done

args=("$minutes")
[ -n "$vent" ] && args+=("vent")

if [ -n "${AFTERBLOW_DRY_RUN:-}" ]; then
	printf 'afterblow %s\n' "${args[*]}"
	exit 0
fi

# 중복 가드: afterblow가 이미 진행 중이면 새 트리거를 무시한다(리스너가 핸들러를
# 백그라운드로 돌리므로 더블탭 시 동시 실행을 막는다). 락은 fd 8을 닫을 때(=프로세스
# 종료/강제종료 시) 자동 해제되므로, 취소가 이 프로세스를 kill하면 곧바로 풀린다.
LOCKFILE="${AFTERBLOW_LOCK:-$ROOT/.afterblow.lock}"
exec 8>"$LOCKFILE"
if ! flock -n 8; then
	log "afterblow 이미 진행 중 — 새 트리거 무시 (${minutes}min, vent=${vent:-off})"
	exit 0
fi
# 취소가 진행 중 afterblow를 찾아 멈출 수 있도록 PID를 남긴다.
PIDFILE="${AFTERBLOW_PIDFILE:-$ROOT/.afterblow.pid}"
printf '%s\n' "$$" >"$PIDFILE"
trap 'rm -f "$PIDFILE"' EXIT

log "AFTERBLOW TRIGGERED (${minutes}min, vent=${vent:-off})"

# AFTERBLOW_CMD로 바이너리를 덮어쓸 수 있다(테스트용; 기본은 실제 tesla-sentry).
CMD="${AFTERBLOW_CMD:-$ROOT/tesla-sentry}"
"$CMD" afterblow "${args[@]}" >>"$LOG" 2>&1
rc=$?
log "command finished (exit $rc)"
