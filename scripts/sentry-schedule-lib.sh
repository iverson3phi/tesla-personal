#!/usr/bin/env bash
# Sentry 스케줄 판정 순수 함수 (부수효과 없음 → 테스트에서 안전하게 source 가능).
# 시각은 zero-padded "HH:MM"(KST) 문자열로 다룬다.
# 슬롯(ON/OFF 한 세트)은 "HH:MM-HH:MM" 문자열이고, [on, off) 구간을 뜻한다.
# off < on 이면 자정을 넘는 구간(예: 22:00-06:00)이다.

# to_min "HH:MM" -> 자정 이후 분(정수)을 출력.
#   10# 접두로 8진수 오해(예: 08, 09)를 막는다.
to_min() {
	local t="$1"
	printf '%s' "$(( 10#${t%%:*} * 60 + 10#${t##*:} ))"
}

# sentry_in_slot <분> <"HH:MM-HH:MM"> -> 분이 슬롯 구간 안이면 return 0.
sentry_in_slot() {
	local t="$1" on off
	on="$(to_min "${2%-*}")"
	off="$(to_min "${2#*-}")"
	if [ "$on" -le "$off" ]; then
		[ "$t" -ge "$on" ] && [ "$t" -lt "$off" ]
	else
		[ "$t" -ge "$on" ] || [ "$t" -lt "$off" ]
	fi
}

# sentry_desired <분> <슬롯...> -> 슬롯 합집합 기준으로 켜져 있어야 하면 return 0.
sentry_desired() {
	local t="$1" s
	shift
	for s in "$@"; do
		sentry_in_slot "$t" "$s" && return 0
	done
	return 1
}

# sentry_action <hhmm> <슬롯...> -> "on" | "off" | "" 출력.
#   합집합이 "꺼짐→켜짐"으로 바뀌는 분에 on, "켜짐→꺼짐"으로 바뀌는 분에 off.
#   systemd timer가 매분 실행하므로 경계 분에만 정확히 한 번 매치된다(상태 가드 불필요).
#   슬롯이 겹치거나 맞닿아도(08-12 + 10-14) 중간에 껐다 켜지 않는다.
sentry_action() {
	local t prev now=0 was=0
	t="$(to_min "$1")"
	shift
	prev=$(( (t + 1439) % 1440 ))
	sentry_desired "$t" "$@" && now=1
	sentry_desired "$prev" "$@" && was=1
	if [ "$now" -eq 1 ] && [ "$was" -eq 0 ]; then
		printf 'on'
	elif [ "$now" -eq 0 ] && [ "$was" -eq 1 ]; then
		printf 'off'
	fi
}
