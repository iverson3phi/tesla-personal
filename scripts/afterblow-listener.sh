#!/usr/bin/env bash
# ntfy 토픽을 상시 구독하다가 "afterblow" 메시지가 오면 핸들러를 실행한다.
# 연결이 끊기면 자동 재접속한다. (outbound 연결만 사용 → 공인 IP 불필요)
set -uo pipefail

TOPIC_URL="https://ntfy.sh/tesla-ab-9f3k7q2zx8m/raw"

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
HANDLER="$DIR/afterblow-run.sh"
LOG="$ROOT/afterblow.log"

log() { printf '%s [listener] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

log "starting (topic=$TOPIC_URL)"
while true; do
	# -s 조용히, -N 버퍼링 끔(스트리밍). ntfy는 빈 줄을 keepalive로 보냄.
	while IFS= read -r line; do
		[ -z "$line" ] && continue # keepalive
		log "recv: $line"
		if [ "$line" = "afterblow" ]; then
			"$HANDLER" || log "handler exited non-zero"
		fi
	done < <(curl -sN "$TOPIC_URL")
	log "stream closed; reconnecting in 5s"
	sleep 5
done
