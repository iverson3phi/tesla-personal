#!/usr/bin/env bash
# Sentry 스케줄 판정 순수 함수 (부수효과 없음 → 테스트에서 안전하게 source 가능).
# 시각은 zero-padded "HH:MM"(KST) 문자열로 다룬다.

# to_min "HH:MM" -> 자정 이후 분(정수)을 출력.
#   10# 접두로 8진수 오해(예: 08, 09)를 막는다.
to_min() {
	local t="$1"
	printf '%s' "$(( 10#${t%%:*} * 60 + 10#${t##*:} ))"
}

# sentry_should_fire <hhmm> <target> -> 현재 시각이 목표 시각과 같으면 return 0.
#   crontab이 매분 실행하므로, 목표 분에만 정확히 한 번 매치된다(상태 가드 불필요).
sentry_should_fire() {
	[ "$(to_min "$1")" -eq "$(to_min "$2")" ]
}
