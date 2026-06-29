# 애프터블로우 트리거 PWA 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** MacroDroid를 대체해, 부부 두 폰에서 건조 시간(1–10분)과 환기 여부를 골라 애프터블로우를 트리거하는 PWA + 그 메시지를 해석하는 PC 수신 스크립트를 만든다.

**Architecture:** 폰의 정적 PWA(Cloudflare Pages)가 `afterblow <분> [vent]` 본문을 ntfy.sh로 POST한다. PC의 기존 셸 스크립트 2개를 공용 파싱 라이브러리(`afterblow-lib.sh`)를 쓰도록 리팩터해, 메시지에서 분/환기를 추출·검증한 뒤 `tesla-sentry afterblow <분> [vent]`로 넘긴다.

**Tech Stack:** 바닐라 HTML/CSS/JS(ES 모듈, 외부 의존성 0), 서비스워커 PWA, Bash, ntfy.sh, Cloudflare Pages(`wrangler`).

## Global Constraints

- ntfy 발행 URL(앱→ntfy): `https://ntfy.sh/tesla-ab-9f3k7q2zx8m` (POST, `Content-Type: text/plain`)
- ntfy 구독 URL(PC): `https://ntfy.sh/tesla-ab-9f3k7q2zx8m/raw`
- 메시지 포맷: `afterblow <분> [vent]` — 분 없으면 기본 3분(하위 호환), 분 범위 **1–10**(범위 밖은 클램프, 숫자 아니면 3)
- 디바운스 기본값: **60초** (환경변수 `DEBOUNCE`로 덮어쓰기 가능)
- 외부 런타임 의존성 추가 금지: 프런트는 바닐라 JS, PC는 Bash만. 빌드 도구는 `npx wrangler`만.
- 새 PWA는 **별도 Cloudflare Pages 프로젝트**(`tesla-afterblow`)로 배포한다 — 기존 공개키 Pages 프로젝트(`.well-known/...`)를 절대 덮어쓰지 않는다.
- 입력은 공개 토픽에서 오므로 신뢰하지 않는다: 분은 PC에서 **반드시 재검증/클램프**.
- **커밋 규칙(중요):** 이 저장소는 `CLAUDE.md`에 따라 사용자가 "커밋 & 푸시"를 명시적으로 요청할 때만 커밋한다. 따라서 각 Task의 마지막 단계는 *커밋*이 아니라 *검증 통과 확인*이다. 커밋은 사용자가 요청할 때 기능 단위로 일괄 수행한다.

---

## 파일 구조

생성/수정할 파일과 책임:

| 파일 | 구분 | 책임 |
|---|---|---|
| `scripts/afterblow-lib.sh` | 생성 | 순수 파싱/검증 함수(부수효과 없음, 소스 가능) |
| `scripts/test-afterblow-parse.sh` | 생성 | lib 단위 테스트 |
| `scripts/afterblow-run.sh` | 수정 | 인자(분/환기)로 동작 + 디바운스 + dry-run |
| `scripts/test-afterblow-run.sh` | 생성 | run.sh dry-run 통합 테스트 |
| `scripts/afterblow-listener.sh` | 수정 | lib로 파싱, 스트림 본체 함수화(소스 가능) |
| `scripts/test-afterblow-listener.sh` | 생성 | listener 스트림 처리 통합 테스트 |
| `webapp/message.js` | 생성 | 메시지 빌드/분 클램프 순수 함수(앱·테스트 공용) |
| `webapp/message.test.mjs` | 생성 | message.js 단위 테스트(node) |
| `webapp/index.html` | 생성 | 화면 마크업 |
| `webapp/style.css` | 생성 | 스타일(다크, 큰 터치 영역) |
| `webapp/app.js` | 생성 | 슬라이더/토글/버튼 → ntfy POST, 결과 표시 |
| `webapp/manifest.json` | 생성 | PWA 매니페스트 |
| `webapp/sw.js` | 생성 | 서비스워커(정적 자원 캐시) |
| `webapp/icon.svg` | 생성 | 앱 아이콘 |
| `README.md` | 수정 | MacroDroid → PWA 문서화, 메시지 포맷 안내 |

---

## Task 1: 파싱 라이브러리 + 단위 테스트

순수 함수로 파싱/검증 로직을 만든다. 이후 listener/run 양쪽이 공유한다(DRY).

**Files:**
- Create: `scripts/afterblow-lib.sh`
- Test: `scripts/test-afterblow-parse.sh`

**Interfaces:**
- Produces:
  - `ab_sanitize_minutes <raw>` → 정수 분 출력(비숫자→`3`, `[1,10]` 클램프)
  - `ab_parse_message <line>` → 첫 토큰이 `afterblow`면 `"<분>"` 또는 `"<분> vent"` 출력 후 return 0, 아니면 return 1
  - `ab_dispatch_line <line> <handler...>` → afterblow 메시지면 핸들러를 파싱 인자로 호출하고 그 종료코드 반환, 아니면 핸들러 호출 없이 return 1

- [ ] **Step 1: 실패하는 테스트 작성**

`scripts/test-afterblow-parse.sh`:

```bash
#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=afterblow-lib.sh
source "$DIR/afterblow-lib.sh"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "sanitize 2"         "2"      "$(ab_sanitize_minutes 2)"
check "sanitize junk -> 3" "3"      "$(ab_sanitize_minutes abc)"
check "sanitize 0 -> 1"    "1"      "$(ab_sanitize_minutes 0)"
check "sanitize 99 -> 10"  "10"     "$(ab_sanitize_minutes 99)"

check "bare afterblow -> 3" "3"      "$(ab_parse_message 'afterblow')"
check "afterblow 2 -> 2"    "2"      "$(ab_parse_message 'afterblow 2')"
check "afterblow 3 vent"    "3 vent" "$(ab_parse_message 'afterblow 3 vent')"
check "afterblow 99 -> 10"  "10"     "$(ab_parse_message 'afterblow 99')"
check "afterblow abc -> 3"  "3"      "$(ab_parse_message 'afterblow abc')"

if ab_parse_message 'hello world' >/dev/null 2>&1; then
	printf 'FAIL - non-afterblow line ignored\n'; fail=1
else
	printf 'ok   - non-afterblow line ignored\n'
fi

got="$(ab_dispatch_line 'afterblow 4 vent' echo)"
check "dispatch args" "4 vent" "$got"

exit "$fail"
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-afterblow-parse.sh`
Expected: 실패 — `afterblow-lib.sh: No such file or directory` (또는 함수 미정의)

- [ ] **Step 3: 라이브러리 구현**

`scripts/afterblow-lib.sh`:

```bash
#!/usr/bin/env bash
# Pure, sourceable helpers for parsing afterblow trigger messages.
# 부수효과 없음(I/O·명령 실행 없음) → 테스트에서 안전하게 source 가능.

# ab_sanitize_minutes <raw> -> 정수 분 출력.
#   - 비숫자 -> 3 (기본)
#   - [1, 10]로 클램프
ab_sanitize_minutes() {
	local raw="${1:-}"
	if ! [[ "$raw" =~ ^[0-9]+$ ]]; then
		printf '3'
		return 0
	fi
	local n=$((10#$raw))
	((n < 1)) && n=1
	((n > 10)) && n=10
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
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `bash scripts/test-afterblow-parse.sh`
Expected: 모든 줄 `ok`, 종료코드 0

- [ ] **Step 5: 검증 통과 확인(커밋 아님)**

위 테스트가 전부 통과하는지 다시 확인한다. (커밋은 Global Constraints의 커밋 규칙에 따라 사용자가 요청할 때 일괄 수행.)

---

## Task 2: `afterblow-run.sh` 리팩터(인자/디바운스/dry-run)

하드코딩된 시간/환기를 인자 기반으로 바꾼다. dry-run으로 테스트 가능하게 한다.

**Files:**
- Modify: `scripts/afterblow-run.sh` (전체 교체)
- Test: `scripts/test-afterblow-run.sh`

**Interfaces:**
- Consumes: `scripts/afterblow-lib.sh`의 `ab_sanitize_minutes`
- Produces: 실행 인터페이스 `afterblow-run.sh <분> [vent]`. `AFTERBLOW_DRY_RUN=1`이면 실제 명령 대신 `afterblow <분> [vent]` 한 줄을 stdout에 출력하고 종료.

- [ ] **Step 1: 실패하는 테스트 작성**

`scripts/test-afterblow-run.sh`:

```bash
#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "run 2 vent"   "afterblow 2 vent" "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 2 vent)"
check "run bare -> 3" "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh")"
check "run 99 -> 10"  "afterblow 10"     "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 99)"
check "run 3 only"    "afterblow 3"      "$(AFTERBLOW_DRY_RUN=1 bash "$DIR/afterblow-run.sh" 3)"

exit "$fail"
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-afterblow-run.sh`
Expected: 실패 — 현재 `afterblow-run.sh`는 인자를 무시하고 `AFTERBLOW_DRY_RUN`도 모르므로 출력이 없음/불일치

- [ ] **Step 3: 구현(파일 전체 교체)**

`scripts/afterblow-run.sh`:

```bash
#!/usr/bin/env bash
# afterblow 트리거 핸들러.
# - 인자로 받은 분/환기로 tesla-sentry afterblow 를 실행한다.
# - 디바운스: 짧은 시간 안의 중복 트리거는 무시한다(실수 더블탭 대비).
# - AFTERBLOW_DRY_RUN=1 이면 실제 명령 대신 해석된 인자를 출력하고 종료(테스트용).
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
# shellcheck source=afterblow-lib.sh
source "$DIR/afterblow-lib.sh"

# ── 설정 (환경변수로 덮어쓸 수 있음) ──────────────────────────────
DEBOUNCE="${DEBOUNCE:-60}" # 초. 이 시간 안의 재트리거는 무시.
LOG="${LOG:-$ROOT/afterblow.log}"
STAMP="${STAMP:-$ROOT/.afterblow-last}"
# ──────────────────────────────────────────────────────────────────

log() { printf '%s [run] %s\n' "$(date '+%F %T')" "$*" >>"$LOG"; }

# 입력 해석(공개 토픽 → 신뢰 불가 입력이므로 재검증).
minutes="$(ab_sanitize_minutes "${1:-}")"
vent=""
for a in "$@"; do [ "$a" = "vent" ] && vent="vent"; done

args=("$minutes")
[ -n "$vent" ] && args+=("vent")

if [ -n "${AFTERBLOW_DRY_RUN:-}" ]; then
	printf 'afterblow %s\n' "${args[*]}"
	exit 0
fi

now=$(date +%s)
if [ -f "$STAMP" ]; then
	last=$(cat "$STAMP" 2>/dev/null || echo 0)
	if [ $((now - last)) -lt "$DEBOUNCE" ]; then
		log "debounced (within ${DEBOUNCE}s) — skip"
		exit 0
	fi
fi
echo "$now" >"$STAMP"

log "AFTERBLOW TRIGGERED (${minutes}min, vent=${vent:-off})"

"$ROOT/tesla-sentry" afterblow "${args[@]}" >>"$LOG" 2>&1
rc=$?
log "command finished (exit $rc)"
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `bash scripts/test-afterblow-run.sh`
Expected: 모든 줄 `ok`, 종료코드 0

- [ ] **Step 5: 디바운스 동작 수동 확인**

Run:
```bash
rm -f /tmp/ab-stamp; STAMP=/tmp/ab-stamp LOG=/tmp/ab-log DEBOUNCE=60 bash scripts/afterblow-run.sh 1 >/dev/null 2>&1; \
STAMP=/tmp/ab-stamp LOG=/tmp/ab-log DEBOUNCE=60 bash scripts/afterblow-run.sh 1 >/dev/null 2>&1; \
grep -c debounced /tmp/ab-log; rm -f /tmp/ab-stamp /tmp/ab-log
```
Expected: `1` (두 번째 호출이 디바운스됨). 주의: 이 단계는 `tesla-sentry`를 실제 호출하므로, 차량이 깨어날 수 있음 — 차를 볼 수 없으면 이 수동 단계는 건너뛰고 Step 4의 dry-run 결과로 대체한다.

- [ ] **Step 6: 검증 통과 확인(커밋 아님)**

Step 4 테스트 통과를 재확인한다.

---

## Task 3: `afterblow-listener.sh` 리팩터(lib 사용 + 스트림 함수화)

정확히 `afterblow`만 매칭하던 것을 lib 파싱으로 바꾸고, 스트림 처리 본체를 함수로 분리해 테스트 가능하게 한다.

**Files:**
- Modify: `scripts/afterblow-listener.sh` (전체 교체)
- Test: `scripts/test-afterblow-listener.sh`

**Interfaces:**
- Consumes: `afterblow-lib.sh`의 `ab_dispatch_line`
- Produces: `ab_consume_stream <handler>` — stdin을 줄 단위로 읽어 afterblow 메시지마다 핸들러를 파싱 인자로 호출. (직접 실행 시에는 `main`이 `curl` 스트림을 이 함수로 흘려보냄.)

- [ ] **Step 1: 실패하는 테스트 작성**

`scripts/test-afterblow-listener.sh`:

```bash
#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LOG=/dev/null
# shellcheck source=afterblow-listener.sh
source "$DIR/afterblow-listener.sh"

RECORD="$(mktemp)"
record() { printf '%s\n' "$*" >>"$RECORD"; }

printf 'afterblow 2 vent\n\nhello world\nafterblow 5\n' | ab_consume_stream record

mapfile -t lines <"$RECORD"
rm -f "$RECORD"

fail=0
check() { # <desc> <expected> <actual>
	if [ "$2" = "$3" ]; then
		printf 'ok   - %s\n' "$1"
	else
		printf 'FAIL - %s (expected [%s] got [%s])\n' "$1" "$2" "$3"
		fail=1
	fi
}

check "count" "2"      "${#lines[@]}"
check "first" "2 vent" "${lines[0]:-}"
check "second" "5"     "${lines[1]:-}"

exit "$fail"
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-afterblow-listener.sh`
Expected: 실패 — 현재 listener에는 `ab_consume_stream` 함수가 없음(`command not found`)

- [ ] **Step 3: 구현(파일 전체 교체)**

`scripts/afterblow-listener.sh`:

```bash
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

# 스트림 본체: stdin을 줄 단위로 읽어 afterblow 메시지마다 핸들러를 호출한다.
# (테스트에서 가짜 입력 + 가짜 핸들러로 호출 가능.)
ab_consume_stream() {
	local handler="$1"
	local line
	while IFS= read -r line; do
		[ -z "$line" ] && continue # keepalive
		log "recv: $line"
		ab_dispatch_line "$line" "$handler" || true
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
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `bash scripts/test-afterblow-listener.sh`
Expected: 모든 줄 `ok`, 종료코드 0

- [ ] **Step 5: 전체 PC 테스트 일괄 실행**

Run: `for t in scripts/test-afterblow-*.sh; do echo "== $t =="; bash "$t" || exit 1; done`
Expected: 세 테스트 모두 `ok`만 출력, 종료코드 0

- [ ] **Step 6: 검증 통과 확인(커밋 아님)**

Step 5 일괄 통과를 재확인한다.

---

## Task 4: PWA 메시지 로직 + 단위 테스트(node)

앱 UI와 분리된 순수 함수로 메시지 생성을 만든다(브라우저·테스트 공용).

**Files:**
- Create: `webapp/message.js`
- Test: `webapp/message.test.mjs`

**Interfaces:**
- Produces:
  - `clampMinutes(raw) -> number` — 비숫자→`3`, 반올림 후 `[1,10]` 클램프
  - `buildMessage(minutes, vent) -> string` — `vent`(boolean) 참이면 `"afterblow <m> vent"`, 아니면 `"afterblow <m>"`

- [ ] **Step 1: 실패하는 테스트 작성**

`webapp/message.test.mjs`:

```js
import assert from 'node:assert/strict';
import { buildMessage, clampMinutes } from './message.js';

assert.equal(clampMinutes(2), 2);
assert.equal(clampMinutes('abc'), 3);
assert.equal(clampMinutes(0), 1);
assert.equal(clampMinutes(99), 10);

assert.equal(buildMessage(2, false), 'afterblow 2');
assert.equal(buildMessage(3, true), 'afterblow 3 vent');
assert.equal(buildMessage(99, true), 'afterblow 10 vent');
assert.equal(buildMessage('abc', false), 'afterblow 3');

console.log('message tests passed');
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `node webapp/message.test.mjs`
Expected: 실패 — `Cannot find module '.../webapp/message.js'`

- [ ] **Step 3: 구현**

`webapp/message.js`:

```js
// 앱 UI와 단위 테스트가 공유하는 순수 함수. 여기서는 DOM에 접근하지 않는다.

// 임의 입력을 [1,10] 정수로 정규화. 숫자가 아니면 기본 3.
export function clampMinutes(raw) {
  const n = Math.round(Number(raw));
  if (!Number.isFinite(n)) return 3;
  return Math.min(10, Math.max(1, n));
}

// PC 리스너가 기대하는 ntfy 본문을 만든다.
export function buildMessage(minutes, vent) {
  const m = clampMinutes(minutes);
  return vent ? `afterblow ${m} vent` : `afterblow ${m}`;
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `node webapp/message.test.mjs`
Expected: `message tests passed`, 종료코드 0

- [ ] **Step 5: 검증 통과 확인(커밋 아님)**

Step 4 통과를 재확인한다.

---

## Task 5: PWA 화면(HTML/CSS/JS)

시간 슬라이더 + 환기 토글 + 시작 버튼 + 결과 표시. `message.js`를 사용한다.

**Files:**
- Create: `webapp/index.html`
- Create: `webapp/style.css`
- Create: `webapp/app.js`

**Interfaces:**
- Consumes: `webapp/message.js`의 `buildMessage`
- Produces: 정적 페이지. 시작 버튼 클릭 시 ntfy로 POST.

- [ ] **Step 1: `webapp/index.html` 작성**

```html
<!DOCTYPE html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <meta name="theme-color" content="#0f172a">
  <title>애프터블로우</title>
  <link rel="manifest" href="./manifest.json">
  <link rel="icon" href="./icon.svg" type="image/svg+xml">
  <link rel="stylesheet" href="./style.css">
</head>
<body>
  <main class="card">
    <h1>애프터블로우</h1>

    <section class="row">
      <span class="label">건조 시간</span>
      <span id="minutesLabel" class="value">3분</span>
    </section>
    <input id="minutes" type="range" min="1" max="10" step="1" value="3" class="slider">

    <section class="row toggle">
      <span class="label">창문 살짝 열기</span>
      <label class="switch">
        <input id="vent" type="checkbox" checked>
        <span class="track"></span>
      </label>
    </section>

    <button id="start" class="start">건조 시작</button>

    <p id="status" class="status"></p>
    <p class="note">‘전송됨’은 메시지 전송 성공이며, 차량 동작 확인은 아닙니다.</p>
  </main>

  <script type="module" src="./app.js"></script>
</body>
</html>
```

- [ ] **Step 2: `webapp/style.css` 작성**

```css
:root {
  --bg: #0f172a;
  --card: #1e293b;
  --fg: #e2e8f0;
  --muted: #94a3b8;
  --accent: #22c55e;
  --accent-press: #16a34a;
  --err: #ef4444;
  --ok: #22c55e;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg);
  color: var(--fg);
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  padding: env(safe-area-inset-top) 16px env(safe-area-inset-bottom);
}

.card {
  width: 100%;
  max-width: 420px;
  background: var(--card);
  border-radius: 20px;
  padding: 28px 24px 24px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, .4);
}

h1 { margin: 0 0 24px; font-size: 1.5rem; text-align: center; }

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.label { color: var(--muted); font-size: 1rem; }
.value { font-size: 1.6rem; font-weight: 700; }

.slider { width: 100%; margin: 4px 0 24px; accent-color: var(--accent); height: 32px; }

.toggle { margin: 8px 0 28px; }

.switch { position: relative; display: inline-block; width: 56px; height: 32px; }
.switch input { opacity: 0; width: 0; height: 0; }
.track {
  position: absolute; inset: 0; cursor: pointer;
  background: #475569; border-radius: 999px; transition: .2s;
}
.track::before {
  content: ""; position: absolute; height: 26px; width: 26px;
  left: 3px; top: 3px; background: #fff; border-radius: 50%; transition: .2s;
}
.switch input:checked + .track { background: var(--accent); }
.switch input:checked + .track::before { transform: translateX(24px); }

.start {
  width: 100%; padding: 18px;
  font-size: 1.25rem; font-weight: 700; color: #fff;
  background: var(--accent); border: none; border-radius: 14px; cursor: pointer;
}
.start:active { background: var(--accent-press); }
.start:disabled { opacity: .6; cursor: default; }

.status { min-height: 1.4em; margin: 16px 0 0; text-align: center; font-weight: 600; }
.status.ok { color: var(--ok); }
.status.err { color: var(--err); }
.status.pending { color: var(--muted); }

.note { margin: 12px 0 0; font-size: .8rem; color: var(--muted); text-align: center; }
```

- [ ] **Step 3: `webapp/app.js` 작성**

```js
import { buildMessage } from './message.js';

const TOPIC_URL = 'https://ntfy.sh/tesla-ab-9f3k7q2zx8m';

const slider = document.getElementById('minutes');
const minutesLabel = document.getElementById('minutesLabel');
const vent = document.getElementById('vent');
const startBtn = document.getElementById('start');
const statusEl = document.getElementById('status');

function renderMinutes() {
  minutesLabel.textContent = `${slider.value}분`;
}
slider.addEventListener('input', renderMinutes);
renderMinutes();

function setStatus(text, kind) {
  statusEl.textContent = text;
  statusEl.className = `status ${kind}`;
}

async function trigger() {
  const body = buildMessage(slider.value, vent.checked);
  startBtn.disabled = true;
  setStatus('전송 중…', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body,
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    setStatus(`전송됨 ✓  (${body})`, 'ok');
  } catch (e) {
    setStatus(`전송 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    startBtn.disabled = false;
  }
}
startBtn.addEventListener('click', trigger);

// 설치형/오프라인 실행을 위한 서비스워커 등록.
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('./sw.js').catch(() => {});
}
```

- [ ] **Step 4: 로컬에서 화면 확인**

Run: `cd webapp && python3 -m http.server 8000` (백그라운드로 띄운 뒤)
브라우저에서 `http://localhost:8000` 열기 → 슬라이더를 움직이면 분 표시가 바뀌고, 토글/버튼이 보이는지 확인. (이 단계에서 실제 전송은 Task 7의 엔드투엔드에서 검증.)
Expected: 다크 카드 UI, 슬라이더 조작 시 "N분" 갱신, 콘솔 에러 없음. 확인 후 서버 종료.

- [ ] **Step 5: 검증 통과 확인(커밋 아님)**

화면이 정상 렌더되고 `message.js`가 모듈로 로드되는지(콘솔 에러 없음) 확인한다.

---

## Task 6: PWA 매니페스트 + 서비스워커 + 아이콘

설치 가능(홈 화면에 추가)하고 오프라인 실행되도록 만든다.

**Files:**
- Create: `webapp/manifest.json`
- Create: `webapp/sw.js`
- Create: `webapp/icon.svg`

**Interfaces:**
- Consumes: Task 5의 정적 자원 목록
- Produces: 설치형 PWA(매니페스트 + SW 캐시)

- [ ] **Step 1: `webapp/icon.svg` 작성**

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <rect width="512" height="512" fill="#0f172a"/>
  <g stroke="#22c55e" stroke-width="22" stroke-linecap="round">
    <line x1="256" y1="140" x2="256" y2="372"/>
    <line x1="140" y1="256" x2="372" y2="256"/>
    <line x1="175" y1="175" x2="337" y2="337"/>
    <line x1="337" y1="175" x2="175" y2="337"/>
  </g>
</svg>
```

- [ ] **Step 2: `webapp/manifest.json` 작성**

```json
{
  "name": "애프터블로우",
  "short_name": "애프터블로우",
  "start_url": "./",
  "scope": "./",
  "display": "standalone",
  "background_color": "#0f172a",
  "theme_color": "#0f172a",
  "icons": [
    { "src": "./icon.svg", "sizes": "any", "type": "image/svg+xml", "purpose": "any maskable" }
  ]
}
```

- [ ] **Step 3: `webapp/sw.js` 작성**

```js
const CACHE = 'afterblow-v1';
const ASSETS = [
  './',
  './index.html',
  './style.css',
  './app.js',
  './message.js',
  './manifest.json',
  './icon.svg',
];

self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(ASSETS)));
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);
  // 우리 정적 자원(GET, 동일 출처)만 캐시에서 제공. ntfy POST는 절대 가로채지 않는다.
  if (e.request.method !== 'GET' || url.origin !== self.location.origin) return;
  e.respondWith(caches.match(e.request).then((cached) => cached || fetch(e.request)));
});
```

- [ ] **Step 4: JSON 유효성 확인**

Run: `node -e "JSON.parse(require('fs').readFileSync('webapp/manifest.json','utf8')); console.log('manifest ok')"`
Expected: `manifest ok`

- [ ] **Step 5: 로컬에서 서비스워커 등록 확인**

Run: `cd webapp && python3 -m http.server 8000` (백그라운드)
브라우저에서 `http://localhost:8000` 열고 DevTools → Application → Service Workers에 `sw.js`가 activated 상태인지, Manifest 탭에 이름/아이콘이 뜨는지 확인. 확인 후 서버 종료.
Expected: SW activated, 매니페스트 인식, 콘솔 에러 없음.

- [ ] **Step 6: 검증 통과 확인(커밋 아님)**

매니페스트/SW가 정상 인식되는지 재확인한다.

---

## Task 7: Cloudflare Pages 배포 + 엔드투엔드 검증 + 문서화

별도 Pages 프로젝트로 배포하고, 폰→ntfy→PC 전체 경로를 검증하고, README를 갱신한다.

**Files:**
- Modify: `README.md`
- (배포 산출물: `tesla-afterblow.pages.dev`)

**Interfaces:**
- Consumes: `webapp/` 전체, Task 1–3의 PC 스크립트

- [ ] **Step 1: 새 Pages 프로젝트 생성(최초 1회)**

Run: `npx wrangler pages project create tesla-afterblow --production-branch=main`
Expected: 프로젝트 생성 성공 메시지. (이미 있으면 "already exists" — 무시하고 진행.)
주의: 기존 공개키 프로젝트와 **다른 이름**임을 확인(덮어쓰기 금지).

- [ ] **Step 2: 배포**

Run: `npx wrangler pages deploy webapp --project-name=tesla-afterblow --branch=main`
Expected: `https://tesla-afterblow.pages.dev` (production) URL 출력.
주의(README의 함정): 반드시 `--branch=main`을 명시해 production으로 올린다. 현재 git 브랜치가 `main`인지 `git branch --show-current`로 먼저 확인.

- [ ] **Step 3: 배포 정상 확인**

Run:
```bash
curl -s -o /dev/null -w '%{http_code}\n' https://tesla-afterblow.pages.dev/
curl -s -o /dev/null -w '%{http_code}\n' https://tesla-afterblow.pages.dev/manifest.json
curl -s -o /dev/null -w '%{http_code}\n' https://tesla-afterblow.pages.dev/message.js
```
Expected: 세 줄 모두 `200`

- [ ] **Step 4: PC 리스너에 새 코드 반영**

Run: `sudo systemctl restart tesla-afterblow && systemctl status tesla-afterblow --no-pager | head -5`
Expected: `active (running)`. (서비스가 `scripts/`의 현재 파일을 실행하므로, 스크립트 변경은 재시작 또는 다음 트리거부터 반영된다. listener 자체 코드가 바뀌었으므로 재시작 필요.)

- [ ] **Step 5: 엔드투엔드 검증(차량 관찰 권장)**

Run(터미널 A): `tail -f afterblow.log`
Run(터미널 B): `rm -f .afterblow-last && curl -d 'afterblow 1' https://ntfy.sh/tesla-ab-9f3k7q2zx8m`
Expected(로그): `recv: afterblow 1` → `AFTERBLOW TRIGGERED (1min, vent=off)` → `command finished (exit 0)`
주의: 실제로 차량 공조가 1분 동작하므로 차를 볼 수 있을 때 수행한다. 폰 앱으로도 동일 검증: 앱에서 1분·환기 OFF로 "건조 시작" → 같은 로그가 찍히는지 확인.

- [ ] **Step 6: 폰에 설치**

두 폰의 Chrome에서 `https://tesla-afterblow.pages.dev` 접속 → 메뉴 → "홈 화면에 추가". 홈 아이콘으로 전체화면 실행되는지 확인.
Expected: 홈 아이콘 실행 시 주소창 없이 앱 화면 표시.

- [ ] **Step 7: README 갱신**

`README.md`의 애프터블로우 "1. 휴대폰 (MacroDroid)" 부분을 PWA 기반으로 교체한다. 다음 내용을 반영:
- MacroDroid 대신 PWA(`https://tesla-afterblow.pages.dev`)를 홈 화면에 추가해 사용.
- 메시지 포맷이 `afterblow <분> [vent]`로 확장됨(분 1–10, 없으면 3분으로 하위 호환).
- 환기 기본값 변화: 분만 보낸 단독 `afterblow`(레거시/테스트)는 이제 **환기 OFF**로 처리됨(환기는 앱 토글로 매번 선택). 기존 `afterblow-run.sh`의 `VENT=1` 고정 설명은 삭제.
- 디바운스 기본값 600초 → 60초로 변경됨.
- 배포 명령: `npx wrangler pages deploy webapp --project-name=tesla-afterblow --branch=main`.
- PC 테스트 실행법: `for t in scripts/test-afterblow-*.sh; do bash "$t"; done` 와 `node webapp/message.test.mjs`.

작성 후 직접 읽어보며 기존 문서 톤/구조와 일관되는지, `afterblow-run.sh` 설정표(`DURATION_MIN`/`VENT` 행)가 새 동작과 모순되지 않는지 확인하고 수정한다.

- [ ] **Step 8: 최종 회귀 확인(커밋 아님)**

Run: `for t in scripts/test-afterblow-*.sh; do echo "== $t =="; bash "$t" || exit 1; done && node webapp/message.test.mjs`
Expected: 모든 테스트 통과. 이후 사용자가 "커밋 & 푸시"를 요청하면 기능 단위로 커밋한다.

---

## Self-Review (작성자 점검 결과)

**Spec coverage:**
- 메시지 포맷/하위호환/분범위/검증 → Task 1, 2 ✓
- 폰 PWA 화면(슬라이더+환기 토글+시작+결과/정직한 문구) → Task 4, 5 ✓
- 설치형/오프라인(매니페스트·SW·아이콘) → Task 6 ✓
- PC listener/run 수정 + 디바운스 60초 → Task 2, 3 ✓
- 배포(별도 Pages 프로젝트)·보안 주의·엔드투엔드·테스트 → Task 7 ✓
- Cloudflare Access(옵션, 미적용) → 의도적으로 범위 외(spec 합의) ✓

**Placeholder scan:** TBD/TODO 없음. 모든 코드 단계에 실제 코드 포함. ✓

**Type/이름 일관성:** `ab_sanitize_minutes`/`ab_parse_message`/`ab_dispatch_line`/`ab_consume_stream`/`run_handler`(bash), `clampMinutes`/`buildMessage`(js), 요소 id(`minutes`/`minutesLabel`/`vent`/`start`/`status`)가 HTML·app.js·SW 자원목록 전반에서 일치. ✓
