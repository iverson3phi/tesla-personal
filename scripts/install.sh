#!/usr/bin/env bash
# tesla-sentry PC 서비스(systemd) 설치·갱신·제거.
#
#   scripts/install.sh              설치/갱신 (재실행 안전 — 코드를 고친 뒤에도 이것만 다시 실행)
#   scripts/install.sh --uninstall  유닛·심링크 제거 (바이너리·설정·토큰·로그는 남김)
#
# 설치 단계:
#   1. go build → <저장소>/tesla-sentry  (데몬 스크립트들이 $ROOT/tesla-sentry 를 호출)
#   2. /usr/local/bin/tesla-sentry → 위 바이너리 심링크 (수동 CLI도 같은 바이너리 사용)
#   3. scripts/systemd/* 템플릿의 @USER@ @HOME@ @ROOT@ 를 치환해 /etc/systemd/system 에 설치
#   4. 예전 crontab 방식의 sentry-schedule-check.sh 줄 제거 (timer와 이중 실행 방지)
#   5. tesla-afterblow.service, tesla-sentry-schedule.timer enable + (재)시작
#
# 일반 사용자로 실행한다 — 빌드는 본인 권한, 시스템 변경만 sudo. 서비스는 실행한
# 사용자(User=)와 그 HOME(~/.config/tesla-sentry)으로 돈다.

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
UNIT_SRC="$DIR/systemd"
UNIT_DST="/etc/systemd/system"
BIN="$ROOT/tesla-sentry"
BIN_LINK="/usr/local/bin/tesla-sentry"
UNITS=(tesla-afterblow.service tesla-sentry-schedule.service tesla-sentry-schedule.timer)
ENABLE=(tesla-afterblow.service tesla-sentry-schedule.timer) # schedule.service는 timer가 실행

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m경고:\033[0m %s\n' "$*" >&2; }
die() {
	printf '\033[31m오류:\033[0m %s\n' "$*" >&2
	exit 1
}

# ── 순수 함수 (scripts/test-install.sh 가 테스트) ─────────────────

# sed 치환 문자열 이스케이프: 구분자 | 와 & \ 를 리터럴로.
sed_escape() { printf '%s' "$1" | sed 's/[|&\\]/\\&/g'; }

# render_unit <템플릿> <user> <home> <root> → stdout
render_unit() {
	sed -e "s|@USER@|$(sed_escape "$2")|g" \
		-e "s|@HOME@|$(sed_escape "$3")|g" \
		-e "s|@ROOT@|$(sed_escape "$4")|g" "$1"
}

# path_ok <경로>: 유닛 파일에 따옴표 없이 넣을 수 있는지(공백 불가).
path_ok() { [[ "$1" != *[[:space:]]* ]]; }

# stdin crontab에서 활성(주석 아닌) sentry-schedule-check.sh 줄 — 제거 대상.
crontab_sentry_lines() { awk '!/^[[:space:]]*#/ && /sentry-schedule-check\.sh/'; }
# stdin crontab에서 위 줄만 뺀 나머지.
crontab_strip() { awk '!/^[[:space:]]*#/ && /sentry-schedule-check\.sh/ {next} {print}'; }

# ── 설치 ──────────────────────────────────────────────────────────

TMPD=""
cleanup() { [ -z "$TMPD" ] || rm -rf "$TMPD"; }

do_install() {
	[ "$(id -u)" -ne 0 ] || die "root로 실행하지 마세요. 일반 사용자로 실행하면 필요한 단계에서 sudo를 요청합니다."
	local user home
	user="$(id -un)"
	home="$HOME"
	path_ok "$ROOT" || die "저장소 경로에 공백이 있으면 안 됩니다: $ROOT"
	path_ok "$home" || die "HOME 경로에 공백이 있으면 안 됩니다: $home"

	local c missing=()
	for c in go curl jq flock sudo systemctl crontab; do
		command -v "$c" >/dev/null || missing+=("$c")
	done
	[ ${#missing[@]} -eq 0 ] || die "필요한 명령이 없습니다: ${missing[*]}"

	say "빌드: $BIN"
	(cd "$ROOT" && go build -o "$BIN" ./cmd/tesla-sentry)

	local cfg="$home/.config/tesla-sentry" f
	for f in config.json token.json private-key.pem; do
		[ -f "$cfg/$f" ] || warn "$cfg/$f 없음 — README '일회성 설정'을 마쳐야 서비스가 실제로 동작합니다."
	done

	say "systemd 유닛 설치 → $UNIT_DST (User=$user, ROOT=$ROOT)"
	TMPD="$(mktemp -d)"
	trap cleanup EXIT
	local u
	for u in "${UNITS[@]}"; do
		render_unit "$UNIT_SRC/$u" "$user" "$home" "$ROOT" >"$TMPD/$u"
	done
	sudo install -m 0644 -t "$UNIT_DST" "${UNITS[@]/#/$TMPD/}"

	if [ -e "$BIN_LINK" ] && [ ! -L "$BIN_LINK" ]; then
		warn "$BIN_LINK 는 예전에 복사 설치한 바이너리입니다 — 저장소 바이너리 심링크로 교체합니다."
	fi
	say "심링크: $BIN_LINK → $BIN"
	sudo ln -sfn "$BIN" "$BIN_LINK"

	local cur old
	if cur="$(crontab -l 2>/dev/null)"; then
		old="$(crontab_sentry_lines <<<"$cur")"
		if [ -n "$old" ]; then
			say "crontab에서 sentry-schedule-check.sh 줄 제거 (systemd timer로 대체):"
			sed 's/^/      - /' <<<"$old"
			crontab_strip <<<"$cur" | crontab -
		fi
	fi

	say "systemd 반영 (daemon-reload, enable, restart)"
	sudo systemctl daemon-reload
	sudo systemctl enable --quiet "${ENABLE[@]}"
	# 갱신 시 새 스크립트·유닛이 반영되도록 재시작(진행 중인 afterblow는 중단된다).
	sudo systemctl restart "${ENABLE[@]}"

	echo
	systemctl --no-pager --lines=0 status tesla-afterblow.service || true
	echo
	systemctl --no-pager list-timers tesla-sentry-schedule.timer || true
	echo
	say "완료. 실행 로그: $ROOT/tesla.log   실패 원인: journalctl -u tesla-afterblow -u tesla-sentry-schedule"
}

# ── 제거 ──────────────────────────────────────────────────────────

do_uninstall() {
	say "서비스 중지·비활성화"
	sudo systemctl disable --now --quiet "${ENABLE[@]}" 2>/dev/null || true
	sudo systemctl stop tesla-sentry-schedule.service 2>/dev/null || true

	say "유닛 제거: $UNIT_DST"
	local u
	for u in "${UNITS[@]}"; do sudo rm -f "$UNIT_DST/$u"; done
	sudo systemctl daemon-reload
	sudo systemctl reset-failed "${UNITS[@]}" 2>/dev/null || true

	# 이 저장소를 가리키는 심링크일 때만 지운다(다른 설치는 건드리지 않음).
	if [ -L "$BIN_LINK" ] && [ "$(readlink "$BIN_LINK")" = "$BIN" ]; then
		say "심링크 제거: $BIN_LINK"
		sudo rm -f "$BIN_LINK"
	fi
	say "완료. 바이너리·설정·토큰·로그는 남겨 두었습니다."
}

usage() {
	cat <<EOF
사용법: scripts/install.sh [--uninstall | --help]
  (옵션 없음)   빌드 + systemd 유닛 설치/갱신 + 서비스 시작 (재실행 안전)
  --uninstall   서비스 중지 + 유닛·심링크 제거 (바이너리·설정·로그 유지)
EOF
}

main() {
	case "${1:-}" in
	"") do_install ;;
	--uninstall) do_uninstall ;;
	-h | --help) usage ;;
	*)
		usage >&2
		exit 2
		;;
	esac
}

# 직접 실행할 때만 main(소스하면 함수만 로드 → 테스트 가능).
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
	set -euo pipefail
	main "$@"
fi
