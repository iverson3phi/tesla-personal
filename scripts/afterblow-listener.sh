#!/usr/bin/env bash
# ntfy 토픽을 상시 구독하다가 "afterblow ..." 메시지가 오면 핸들러를 실행한다.
# 연결이 끊기면 자동 재접속한다. (outbound 연결만 사용 → 공인 IP 불필요)
set -uo pipefail

TOPIC_URL="https://ntfy.sh/tesla-ab-9f3k7q2zx8m/raw"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
HANDLER="$DIR/afterblow-run.sh"
LOG="${LOG:-$ROOT/afterblow.log}"
# shellcheck source=afterblow-lib.sh
source "$DIR/afterblow-lib.sh"

log() { printf '%s [listener] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

# 핸들러 호출 래퍼(비정상 종료를 로그에 남김).
run_handler() { "$HANDLER" "$@" || log "handler exited non-zero"; }

# sentry on|off 실행 래퍼.
run_sentry() { "$ROOT/tesla-sentry" "$1" >>"$LOG" 2>&1 || log "tesla-sentry $1 exited non-zero"; }

# 스트림 본체: stdin을 줄 단위로 읽어 afterblow 메시지마다 핸들러를 호출한다.
# (테스트에서 가짜 입력 + 가짜 핸들러로 호출 가능.)
ab_consume_stream() {
	local handler="$1"
	local line
	while IFS= read -r line; do
		[ -z "$line" ] && continue # keepalive
		log "recv: $line"
		local sentry_arg
		if sentry_arg="$(ab_parse_sentry "$line")"; then
			log "sentry $sentry_arg"
			run_sentry "$sentry_arg"
		else
			ab_dispatch_line "$line" "$handler" || true
		fi
	done
}

main() {
	log "starting (topic=$TOPIC_URL)"
	while true; do
		# -s 조용히, -N 버퍼링 끔(스트리밍). ntfy는 빈 줄을 keepalive로 보냄.
		ab_consume_stream run_handler < <(curl -sN "$TOPIC_URL")
		log "stream closed; reconnecting in 5s"
		sleep 5
	done
}

# 직접 실행할 때만 main 루프 시작(소스하면 함수만 로드 → 테스트 가능).
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
	main
fi
