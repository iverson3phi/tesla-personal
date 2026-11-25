#!/usr/bin/env bash
# systemd timer(tesla-sentry-schedule.timer)가 매분 0초에 실행한다. 설치: scripts/install.sh
# Worker(KV)에서 ON/OFF 시각 세트(slots, 여러 개)·활성 토글을 읽어, 현재 KST 분이
# 슬롯 합집합의 경계(꺼짐→켜짐 / 켜짐→꺼짐)이면 tesla-sentry on/off 를 실행한다.
# 시각은 폰 PWA에서 KV로 설정되므로 이 스크립트(타이머)는 고정이다.
#
# 정시 판정이라 상태 가드가 필요 없다(경계 분에만 매치되어 1회 실행).
# 트레이드오프: 그 1분에 PC가 꺼져 있으면 그날은 건너뛴다.
#
# Cloudflare Worker Cron 장애(2026-06) 우회: 매분 타이머를 PC가 담당한다
# (처음엔 crontab, 현재는 systemd timer).
#
# 환경변수: SENTRY_DRY_RUN=1  실제 명령 대신 의도된 동작만 로그(테스트용).
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
# shellcheck source=sentry-schedule-lib.sh
source "$DIR/sentry-schedule-lib.sh"

# ── 설정 (환경변수로 덮어쓸 수 있음) ──────────────────────────────
SENTRY_API="${SENTRY_API:-https://tesla-sentry-scheduler.yhlee512.workers.dev/api/sentry-schedule}"
STATE_DIR="${STATE_DIR:-$HOME/.config/tesla-sentry}"
LOG="${LOG:-$ROOT/tesla.log}"
LOCK="$STATE_DIR/.sentry-check.lock"
# ──────────────────────────────────────────────────────────────────

log() { printf '%s [sched] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

run_sentry() { # <on|off>
	if [ -n "${SENTRY_DRY_RUN:-}" ]; then
		log "[dry-run] tesla-sentry $1"
		return 0
	fi
	"$ROOT/tesla-sentry" "$1" >>"$LOG" 2>&1
}

# 이전 실행(차 깨우기 등으로 수십 초 소요 가능)이 안 끝났으면 이번 분은 건너뛴다.
# (timer는 실행 중이면 새로 시작하지 않지만, 수동 실행과 겹치는 경우를 위해 유지.)
exec 9>"$LOCK"
flock -n 9 || exit 0

# KV에서 현재 스케줄 읽기 (GET은 공개라 토큰 불필요).
json="$(curl -s --max-time 10 "$SENTRY_API")" || { log "KV GET 실패(네트워크)"; exit 0; }
enabled="$(printf '%s' "$json" | jq -r '.enabled // empty' 2>/dev/null)"
# slots 배열을 "HH:MM-HH:MM" 줄로. 구버전 Worker(onTime/offTime 단일 세트)도 받아준다.
mapfile -t raw_slots < <(printf '%s' "$json" | jq -r '
	(.slots // (if .onTime then [{onTime, offTime}] else [] end))[]
	| "\(.onTime)-\(.offTime)"' 2>/dev/null)

# 형식이 이상한 슬롯은 로그만 남기고 건너뛴다.
HHMM='([01][0-9]|2[0-3]):[0-5][0-9]'
slots=()
for s in "${raw_slots[@]}"; do
	if [[ "$s" =~ ^${HHMM}-${HHMM}$ ]]; then slots+=("$s"); else log "슬롯 형식 이상: '$s'"; fi
done
# 응답이 비정상이면 조용히 종료(다음 분에 재시도).
[ "${#slots[@]}" -gt 0 ] || { log "유효한 슬롯 없음 (응답 이상)"; exit 0; }

# 마스터 토글: 자동화 정지면 아무것도 안 함(강제 OFF 아님).
[ "$enabled" = "true" ] || exit 0

hhmm="$(date +%H:%M)"

action="$(sentry_action "$hhmm" "${slots[@]}")"
if [ -n "$action" ]; then
	log "SENTRY ${action^^} 트리거 (now=$hhmm slots=${slots[*]})"
	run_sentry "$action" && log "sentry $action 완료" || log "sentry $action 실패"
fi
