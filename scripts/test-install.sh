#!/usr/bin/env bash
# install.sh 순수 함수 단위 테스트 (sudo/systemctl 호출 없음).
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=install.sh
source "$DIR/install.sh"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}
line() { grep -m1 "^$1" <<<"$2"; } # <prefix> <text> → 첫 매치 줄

# ── render_unit: @USER@ @HOME@ @ROOT@ 치환 ─────────────────────────
ab="$(render_unit "$DIR/systemd/tesla-afterblow.service" alice /home/alice /opt/tesla)"
check "afterblow User"      "User=alice"                                   "$(line User= "$ab")"
check "afterblow HOME"      "Environment=HOME=/home/alice"                 "$(line Environment=HOME= "$ab")"
check "afterblow ExecStart" "ExecStart=/opt/tesla/scripts/afterblow-listener.sh" "$(line ExecStart= "$ab")"
check "afterblow Restart"   "Restart=always"                               "$(line Restart= "$ab")"

sv="$(render_unit "$DIR/systemd/tesla-sentry-schedule.service" alice /home/alice /opt/tesla)"
check "schedule Type"       "Type=oneshot"                                 "$(line Type= "$sv")"
check "schedule User"       "User=alice"                                   "$(line User= "$sv")"
check "schedule HOME"       "Environment=HOME=/home/alice"                 "$(line Environment=HOME= "$sv")"
check "schedule ExecStart"  "ExecStart=/opt/tesla/scripts/sentry-schedule-check.sh" "$(line ExecStart= "$sv")"

tm="$(render_unit "$DIR/systemd/tesla-sentry-schedule.timer" alice /home/alice /opt/tesla)"
check "timer 매분 0초"       "OnCalendar=*-*-* *:*:00"                      "$(line OnCalendar= "$tm")"
check "timer 정확도 1s"      "AccuracySec=1s"                               "$(line AccuracySec= "$tm")"

for u in "$DIR"/systemd/*; do
	out="$(render_unit "$u" alice /home/alice /opt/tesla)"
	check "자리표시자 없음: $(basename "$u")" "" "$(grep -oE '@[A-Z]+@' <<<"$out")"
done

# sed 특수문자(&, |, \)가 경로에 있어도 그대로 들어가야 한다.
odd="$(render_unit "$DIR/systemd/tesla-afterblow.service" bob /home/bob '/opt/a&b|c\d')"
check "특수문자 경로" 'ExecStart=/opt/a&b|c\d/scripts/afterblow-listener.sh' "$(line ExecStart= "$odd")"

# ── path_ok: 유닛에 따옴표 없이 넣을 수 있는 경로(공백 불가) ──────
po() { path_ok "$1" && echo ok || echo no; }
check "path_ok 일반"   "ok" "$(po /home/allen/Projects/tesla-personal)"
check "path_ok 공백"   "no" "$(po '/home/allen/My Projects/tesla')"
check "path_ok 탭"     "no" "$(po $'/home/allen/a\tb')"

# ── crontab_strip: 활성 sentry-schedule-check.sh 줄만 제거 ──────────
cron_in=$'#30 9-15 * * 1-5 python3 /x/portfolio.py -t -r
* * * * * /home/allen/Projects/tesla/scripts/sentry-schedule-check.sh
# * * * * * /old/scripts/sentry-schedule-check.sh
0 5 * * 1 tar -zcf /var/backups/home.tgz /home/'
cron_want=$'#30 9-15 * * 1-5 python3 /x/portfolio.py -t -r
# * * * * * /old/scripts/sentry-schedule-check.sh
0 5 * * 1 tar -zcf /var/backups/home.tgz /home/'
check "crontab_strip 활성 줄만 제거" "$cron_want" "$(crontab_strip <<<"$cron_in")"
check "crontab_strip 해당 없음 → 그대로" "0 5 * * 1 backup" "$(crontab_strip <<<'0 5 * * 1 backup')"
check "crontab_sentry_lines" "* * * * * /home/allen/Projects/tesla/scripts/sentry-schedule-check.sh" \
	"$(crontab_sentry_lines <<<"$cron_in")"

exit "$fail"
