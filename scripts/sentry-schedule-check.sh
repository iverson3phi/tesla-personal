#!/usr/bin/env bash
# crontab(* * * * *)이 매분 실행한다.
# Worker(KV)에서 ON/OFF 시각·활성 토글을 읽어, 현재 KST 시각이 목표 시각과
# "정확히 같은 분"이면 tesla-sentry on/off 를 실행한다. 시각은 폰 PWA에서 KV로
# 설정되므로 이 스크립트(crontab 줄)는 고정이다 — "시각 하드코딩" 문제가 없다.
#
# 정시(==) 판정이라 상태 가드가 필요 없다(그 분에만 매치되어 1회 실행).
# 트레이드오프: 그 1분에 PC가 꺼져 있으면 그날은 건너뛴다.
#
# Cloudflare Worker Cron 장애(2026-06) 우회: 매분 타이머를 PC cron이 담당한다.
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
LOG="${LOG:-$STATE_DIR/sentry.log}"
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
exec 9>"$LOCK"
flock -n 9 || exit 0

# KV에서 현재 스케줄 읽기 (GET은 공개라 토큰 불필요).
json="$(curl -s --max-time 10 "$SENTRY_API")" || { log "KV GET 실패(네트워크)"; exit 0; }
enabled="$(printf '%s' "$json" | jq -r '.enabled // empty' 2>/dev/null)"
onTime="$(printf '%s' "$json" | jq -r '.onTime // empty' 2>/dev/null)"
offTime="$(printf '%s' "$json" | jq -r '.offTime // empty' 2>/dev/null)"

# 응답이 비정상이면 조용히 종료(다음 분에 재시도).
[[ "$onTime"  =~ ^([01][0-9]|2[0-3]):[0-5][0-9]$ ]] || { log "onTime 형식 이상: '$onTime'"; exit 0; }
[[ "$offTime" =~ ^([01][0-9]|2[0-3]):[0-5][0-9]$ ]] || { log "offTime 형식 이상: '$offTime'"; exit 0; }

# 마스터 토글: 자동화 정지면 아무것도 안 함(강제 OFF 아님).
[ "$enabled" = "true" ] || exit 0

hhmm="$(date +%H:%M)"

if sentry_should_fire "$hhmm" "$onTime"; then
	log "SENTRY ON 트리거 (now=$hhmm onTime=$onTime)"
	run_sentry on && log "sentry on 완료" || log "sentry on 실패"
fi

if sentry_should_fire "$hhmm" "$offTime"; then
	log "SENTRY OFF 트리거 (now=$hhmm offTime=$offTime)"
	run_sentry off && log "sentry off 완료" || log "sentry off 실패"
fi
