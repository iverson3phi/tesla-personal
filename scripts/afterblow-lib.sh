#!/usr/bin/env bash
# Pure, sourceable helpers for parsing afterblow trigger messages.
# 부수효과 없음(I/O·명령 실행 없음) → 테스트에서 안전하게 source 가능.

# ab_sanitize_minutes <raw> -> 정수 분 출력.
#   - 비숫자 -> 3 (기본)
#   - [1, 3]로 클램프
ab_sanitize_minutes() {
	local raw="${1:-}"
	if ! [[ "$raw" =~ ^[0-9]+$ ]]; then
		printf '3'
		return 0
	fi
	local n=$((10#$raw))
	((n < 1)) && n=1
	((n > 3)) && n=3
	printf '%s' "$n"
}

# ab_parse_message <line> -> 첫 토큰이 "afterblow"면 핸들러 인자를 출력하고 return 0.
#   출력: "<분>" 또는 "<분> vent". 아니면 return 1 (우리 메시지가 아님).
ab_parse_message() {
	local line="$1"
	local toks
	read -r -a toks <<<"$line"
	[ "${toks[0]:-}" = "afterblow" ] || return 1
	local minutes vent="" t
	minutes="$(ab_sanitize_minutes "${toks[1]:-}")"
	for t in "${toks[@]:1}"; do
		[ "$t" = "vent" ] && vent="vent"
	done
	if [ -n "$vent" ]; then
		printf '%s %s' "$minutes" "$vent"
	else
		printf '%s' "$minutes"
	fi
}

# ab_dispatch_line <line> <handler...> -> afterblow 메시지면 핸들러를 파싱 인자로
#   호출하고 그 종료코드를 반환. 아니면 핸들러 호출 없이 return 1.
ab_dispatch_line() {
	local line="$1"
	shift
	local parsed
	parsed="$(ab_parse_message "$line")" || return 1
	# 의도적 비따옴표: parsed는 정수+리터럴 "vent"로 이미 정제됨.
	"$@" $parsed
}

# ab_parse_sentry <line> -> "sentry on|off" 메시지면 on/off 토큰을 출력하고 return 0.
#   정확히 2토큰(sentry + on|off)만 허용. 아니면 return 1 (우리 메시지가 아님).
ab_parse_sentry() {
	local line="$1"
	local toks
	read -r -a toks <<<"$line"
	[ "${#toks[@]}" -eq 2 ] || return 1
	[ "${toks[0]}" = "sentry" ] || return 1
	case "${toks[1]}" in
		on | off) printf '%s' "${toks[1]}" ;;
		*) return 1 ;;
	esac
}
