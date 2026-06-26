#!/usr/bin/env bash
# afterblow 트리거 핸들러.
# - 디바운스: 짧은 시간 안에 중복 트리거되면 무시한다(BT 순간 끊김 대비).
# - STEP 2: 지금은 로그만 남긴다(전체 경로 검증용).
# - STEP 3: 아래 주석을 풀어 실제 히터MAX 명령을 실행한다.
set -uo pipefail

# ── 설정 ──────────────────────────────────────────────────────────
DURATION_MIN=3 # 건조 시간(분). 원하는 값으로 조절.
VENT=0         # 1이면 건조 중 창문 살짝 열기(환기). 보안/비 주의.
DEBOUNCE=600   # 초. 이 시간 안의 재트리거는 무시(BT 순간 끊김 대비).
# ──────────────────────────────────────────────────────────────────

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
LOG="$ROOT/afterblow.log"
STAMP="$ROOT/.afterblow-last"

log() { printf '%s [run] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

now=$(date +%s)
if [ -f "$STAMP" ]; then
	last=$(cat "$STAMP" 2>/dev/null || echo 0)
	if [ $((now - last)) -lt "$DEBOUNCE" ]; then
		log "debounced (within ${DEBOUNCE}s) — skip"
		exit 0
	fi
fi
echo "$now" >"$STAMP"

log "AFTERBLOW TRIGGERED (${DURATION_MIN}min, vent=${VENT})"

args=("$DURATION_MIN")
[ "$VENT" = "1" ] && args+=("vent")

"$ROOT/tesla-sentry" afterblow "${args[@]}" >>"$LOG" 2>&1
rc=$?
log "command finished (exit $rc)"
