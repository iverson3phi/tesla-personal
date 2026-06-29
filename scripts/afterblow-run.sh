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
DEBOUNCE="${DEBOUNCE:-60}" # 초. 이 시간 안의 재트리거는 무시.
LOG="${LOG:-$ROOT/afterblow.log}"
STAMP="${STAMP:-$ROOT/.afterblow-last}"
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

now=$(date +%s)
if [ -f "$STAMP" ]; then
	last=$(cat "$STAMP" 2>/dev/null || echo 0)
	if [ $((now - last)) -lt "$DEBOUNCE" ]; then
		log "debounced (within ${DEBOUNCE}s) — skip"
		exit 0
	fi
fi
echo "$now" >"$STAMP"

log "AFTERBLOW TRIGGERED (${minutes}min, vent=${vent:-off})"

"$ROOT/tesla-sentry" afterblow "${args[@]}" >>"$LOG" 2>&1
rc=$?
log "command finished (exit $rc)"
