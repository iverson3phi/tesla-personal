# 테슬라 4기능 (로그 통합·취소·기본값1분·감시상태) 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 로그를 프로젝트 루트 `tesla.log`로 통합하고, 애프터블로우 취소 버튼·폰 기본값 1분·감시모드 on/off 상태 표시(마지막 명령 + 실시간 확인)를 추가한다.

**Architecture:** 기존 4갈래 구조(Go 바이너리 `tesla-sentry` / bash 스크립트 / Cloudflare Worker+KV / PWA)를 그대로 따른다. 상태 보고는 Go 바이너리가 명령 성공 후 Worker에 PUT하는 방식으로 단일화한다. 폰의 실시간 확인은 ntfy→PC→KV→폰 폴링의 비동기 라운드트립이다.

**Tech Stack:** Go 1.23 (`teslamotors/vehicle-command v0.4.1`), bash, Cloudflare Workers(KV), 바닐라 JS PWA, ntfy.sh.

## Global Constraints

- **테스트 실행 명령**: Go = `go test ./...` / bash = `bash scripts/<name>.sh` / JS = `node <path>.test.mjs` (package.json 없음, 순수 노드 실행).
- **주석/문구는 한국어** (기존 코드 컨벤션).
- **커밋 정책 (CLAUDE.md 우선)**: 사용자가 "커밋 & 푸시"를 명시 요청할 때만 커밋한다. 각 태스크 끝의 **Commit 스텝은 사용자가 요청할 때만 실행**하고, 평소엔 변경을 작업트리에 둔 채 다음 태스크로 진행한다. (계획에는 커밋 메시지를 적어두되 자동 실행하지 않는다.)
- **상수는 기존 값 재사용**: ntfy 토픽 `https://ntfy.sh/tesla-ab-9f3k7q2zx8m`(webapp는 끝에 슬래시 없음, listener는 `/raw`), Worker `https://tesla-sentry-scheduler.yhlee512.workers.dev`, `SENTRY_API=.../api/sentry-schedule`.
- **로그 파일 단일화**: 모든 스크립트의 로그는 `프로젝트루트/tesla.log` 기본값으로 모은다. 설정/토큰/키/락 등 비로그 파일은 `~/.config/tesla-sentry/`에 그대로 둔다.
- **순수 함수만 단위 테스트**: 기존 관례상 `app.js`(DOM/fetch)·`worker/src/index.js`(fetch 핸들러)는 단위 테스트가 없다. 신규 로직도 순수 헬퍼만 테스트하고, 통합부는 수동 검증한다.

---

## File Structure (변경 맵)

**기능 1 — 로그 통합**
- Modify: `scripts/afterblow-run.sh`, `scripts/afterblow-listener.sh`, `scripts/sentry-schedule-check.sh` (LOG 기본값)
- Modify: `.gitignore`, `README.md`

**기능 2 — 취소**
- Modify: `internal/tesla/command.go` (`AfterBlowCancel`)
- Create: `internal/tesla/command_cancel_test.go`
- Modify: `cmd/tesla-sentry/main.go` (`afterblow-cancel` 서브커맨드 + usage)
- Modify: `scripts/afterblow-lib.sh` (`ab_parse_cancel`)
- Modify: `scripts/test-afterblow-parse.sh` (cancel 케이스)
- Modify: `scripts/afterblow-listener.sh` (cancel 분기 + `run_cancel`)
- Modify: `scripts/test-afterblow-listener.sh` (cancel/sentry 스텁 테스트)
- Modify: `webapp/message.js` (`buildCancelMessage`), `webapp/message.test.mjs`
- Modify: `webapp/index.html` (취소 버튼), `webapp/app.js` (취소 핸들러)

**기능 3 — 기본값 1분**
- Modify: `webapp/index.html` (슬라이더 value/라벨)

**기능 4 — 감시모드 상태 표시**
- Modify: `worker/src/schedule.js` (`DEFAULT_SCHEDULE` 필드 + `validateStateInput`)
- Modify: `worker/src/schedule.test.mjs` (검증/기본값 테스트)
- Modify: `worker/src/index.js` (`PUT /api/sentry-state` 라우트)
- Modify: `internal/config/config.go` (`SentryStateURL`/`SentryStateToken`)
- Create: `internal/tesla/report.go` (`ReportSentryState`), `internal/tesla/report_test.go`
- Modify: `cmd/tesla-sentry/main.go` (보고 호출)
- Modify: `scripts/afterblow-lib.sh` (`ab_parse_sentry`에 `status` 허용), `scripts/test-sentry-parse.sh`
- Modify: `webapp/sentry.js` (`buildSentryStatusMessage`), `webapp/sentry.test.mjs`
- Modify: `webapp/index.html` (상태 표시 + 실시간 버튼), `webapp/app.js` (로드 표시 + 폴링)

---

## Task 1: 로그를 `tesla.log`로 통합

**Files:**
- Modify: `scripts/afterblow-run.sh` (LOG 기본값 줄)
- Modify: `scripts/afterblow-listener.sh` (LOG 기본값 줄)
- Modify: `scripts/sentry-schedule-check.sh` (LOG 기본값 줄)
- Modify: `.gitignore`
- Modify: `README.md`

**Interfaces:**
- Produces: 런타임 로그가 모두 `$ROOT/tesla.log`로 감. 다른 태스크와 코드 의존 없음.

- [ ] **Step 1: afterblow-run.sh 의 LOG 기본값 변경**

`scripts/afterblow-run.sh` 의 다음 줄
```bash
LOG="${LOG:-$ROOT/afterblow.log}"
```
을 아래로 변경:
```bash
LOG="${LOG:-$ROOT/tesla.log}"
```

- [ ] **Step 2: afterblow-listener.sh 의 LOG 기본값 변경**

`scripts/afterblow-listener.sh` 의 다음 줄
```bash
LOG="${LOG:-$ROOT/afterblow.log}"
```
을 아래로 변경:
```bash
LOG="${LOG:-$ROOT/tesla.log}"
```

- [ ] **Step 3: sentry-schedule-check.sh 의 LOG 기본값 변경**

`scripts/sentry-schedule-check.sh` 의 다음 줄
```bash
LOG="${LOG:-$STATE_DIR/sentry.log}"
```
을 아래로 변경 (락 파일 `LOCK="$STATE_DIR/.sentry-check.lock"` 줄은 그대로 둔다):
```bash
LOG="${LOG:-$ROOT/tesla.log}"
```

- [ ] **Step 4: .gitignore 갱신**

`.gitignore` 의 `afterblow.log` 줄을 `tesla.log` 로 변경:
```
# afterblow 런타임 파일
tesla.log
.afterblow-last
```

- [ ] **Step 5: README.md 의 로그 위치 설명 갱신**

`README.md` 에서 `sentry.log`/`afterblow.log`/`~/.config/tesla-sentry/sentry.log` 로그 경로 언급을 찾아 "로그는 프로젝트 루트 `tesla.log` 한 파일에 모인다(`[run]`/`[listener]`/`[sched]` 태그로 구분)"는 취지로 수정한다. (검색: `grep -n "sentry.log\|afterblow.log" README.md`)

- [ ] **Step 6: 기존 스크립트 테스트가 통과하는지 확인**

Run:
```bash
for t in scripts/test-afterblow-*.sh scripts/test-sentry-*.sh; do echo "== $t =="; bash "$t"; done
```
Expected: 모든 라인 `ok`, 종료코드 0. (테스트는 `LOG` 환경변수를 주입하거나 `/dev/null`을 쓰므로 기본값 변경에 영향 없음.)

- [ ] **Step 7: (요청 시) 커밋**

```bash
git add scripts/afterblow-run.sh scripts/afterblow-listener.sh scripts/sentry-schedule-check.sh .gitignore README.md
git commit -m "feat: 모든 런타임 로그를 프로젝트 루트 tesla.log로 통합"
```

---

## Task 2: 폰 애프터블로우 기본값 1분

**Files:**
- Modify: `webapp/index.html`

**Interfaces:**
- Produces: PWA 슬라이더 초기값 1분. 코드 의존 없음.

- [ ] **Step 1: 슬라이더 초기 라벨 변경**

`webapp/index.html` 의
```html
      <span id="minutesLabel" class="value">3분</span>
```
을
```html
      <span id="minutesLabel" class="value">1분</span>
```

- [ ] **Step 2: 슬라이더 기본값 변경**

`webapp/index.html` 의
```html
    <input id="minutes" type="range" min="1" max="3" step="1" value="3" class="slider">
```
을
```html
    <input id="minutes" type="range" min="1" max="3" step="1" value="1" class="slider">
```

- [ ] **Step 3: 수동 확인**

`webapp/index.html` 을 브라우저로 열어 페이지 로드시 라벨이 "1분"이고 슬라이더가 최좌측(1)인지 확인. (`app.js`의 `renderMinutes`는 `slider.value`를 읽으므로 슬라이더 조작 시 라벨이 따라감.)

- [ ] **Step 4: (요청 시) 커밋**

```bash
git add webapp/index.html
git commit -m "feat: 애프터블로우 폰 기본 건조 시간을 1분으로 변경"
```

---

## Task 3: Go `AfterBlowCancel` 함수

**Files:**
- Modify: `internal/tesla/command.go`
- Test: `internal/tesla/command_cancel_test.go` (Create)

**Interfaces:**
- Consumes: `withVehicle(ctx, accessToken, vin, privateKeyPath, fn)` (command.go 기존), `car.ClimateOff(ctx)`, `car.CloseWindows(ctx)` (SDK).
- Produces: `func AfterBlowCancel(ctx context.Context, accessToken, vin, privateKeyPath string) error` — Task 4(main.go)가 호출.

- [ ] **Step 1: 실패하는 테스트 작성**

`AfterBlowCancel`은 SDK `*vehicle.Vehicle`을 실차에 연결하므로 단위 테스트로 동작 전체를 검증하긴 어렵다. 대신 **함수 시그니처/존재**를 컴파일 타임에 보장하는 가벼운 테스트를 둔다. `internal/tesla/command_cancel_test.go` 생성:
```go
package tesla

import (
	"context"
	"testing"
	"time"
)

// AfterBlowCancel의 시그니처를 고정한다(컴파일 가드). 실제 차량 호출은
// 자격증명/네트워크가 없으면 에러를 반환하므로, 여기서는 nil이 아닌 에러로
// 빠르게 끝나는지(=함수가 존재하고 호출 가능)만 확인한다.
func TestAfterBlowCancelSignature(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// 존재하지 않는 키 경로 → 즉시 에러. 패닉/컴파일에러가 없으면 통과.
	if err := AfterBlowCancel(ctx, "AT", "VIN", "/nonexistent/key.pem"); err == nil {
		t.Fatal("expected error with bogus key path")
	}
}
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/tesla/ -run TestAfterBlowCancelSignature -v`
Expected: FAIL — `undefined: AfterBlowCancel` (컴파일 에러).

- [ ] **Step 3: `AfterBlowCancel` 구현**

`internal/tesla/command.go` 의 `AfterBlow` 함수 끝(파일 맨 아래) 뒤에 추가:
```go
// AfterBlowCancel는 진행 중인 애프터블로우를 즉시 되돌린다: 공조를 끄고 창문을
// 닫는다. 실행 중이던 afterblow 프로세스를 죽이지 않는 독립 명령이므로, 남은
// 프로세스가 나중에 같은 종료 단계를 또 수행해도 무해(이미 꺼짐/닫힘)하다.
// 모든 단계를 best-effort로 시도하고 발생한 에러를 합쳐 반환한다.
func AfterBlowCancel(ctx context.Context, accessToken, vin, privateKeyPath string) error {
	return withVehicle(ctx, accessToken, vin, privateKeyPath, func(car *vehicle.Vehicle) error {
		var errs []error
		if err := car.ClimateOff(ctx); err != nil {
			errs = append(errs, fmt.Errorf("climate off: %w", err))
		}
		if err := car.CloseWindows(ctx); err != nil {
			errs = append(errs, fmt.Errorf("close windows: %w", err))
		}
		return errors.Join(errs...)
	})
}
```
(`errors`, `fmt`, `vehicle`는 command.go가 이미 import 중.)

- [ ] **Step 4: 테스트 통과 확인**

Run: `go test ./internal/tesla/ -run TestAfterBlowCancelSignature -v`
Expected: PASS.

- [ ] **Step 5: 전체 Go 테스트 회귀 확인**

Run: `go test ./...`
Expected: 모든 패키지 ok.

- [ ] **Step 6: (요청 시) 커밋**

```bash
git add internal/tesla/command.go internal/tesla/command_cancel_test.go
git commit -m "feat: 애프터블로우 취소용 AfterBlowCancel(공조off+창문닫기) 추가"
```

---

## Task 4: `afterblow-cancel` 서브커맨드 연결

**Files:**
- Modify: `cmd/tesla-sentry/main.go`

**Interfaces:**
- Consumes: `tesla.AfterBlowCancel(...)` (Task 3), `loadForCommand(ctx)`, `config.Path(...)`, `commandTimeout` (기존).
- Produces: CLI `tesla-sentry afterblow-cancel` — Task 7(listener)이 실행.

- [ ] **Step 1: usage 문자열 갱신**

`cmd/tesla-sentry/main.go` 의 `usage()`:
```go
	fmt.Fprintln(os.Stderr, "usage: tesla-sentry <keygen|register|login|on|off|status|afterblow [minutes] [vent]>")
```
을:
```go
	fmt.Fprintln(os.Stderr, "usage: tesla-sentry <keygen|register|login|on|off|status|afterblow [minutes] [vent]|afterblow-cancel>")
```

- [ ] **Step 2: switch에 케이스 추가**

`run()` 의 switch에서 `case "afterblow":` 바로 아래에 추가:
```go
	case "afterblow-cancel":
		return cmdAfterBlowCancel()
```

- [ ] **Step 3: `cmdAfterBlowCancel` 구현**

`cmd/tesla-sentry/main.go` 의 `cmdAfterBlow` 함수 뒤에 추가:
```go
// cmdAfterBlowCancel은 진행 중인 애프터블로우를 즉시 되돌린다(공조off+창문닫기).
func cmdAfterBlowCancel() error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cfg, at, err := loadForCommand(ctx)
	if err != nil {
		return err
	}
	priv, err := config.Path("private-key.pem")
	if err != nil {
		return err
	}
	log.Printf("after-blow: cancel (climate off + close windows)")
	if err := tesla.AfterBlowCancel(ctx, at, cfg.VIN, priv); err != nil {
		return err
	}
	log.Printf("after-blow: cancel done")
	return nil
}
```

- [ ] **Step 4: 빌드/베팅 확인**

Run: `go build ./... && go vet ./...`
Expected: 에러 없음.

- [ ] **Step 5: 서브커맨드 인식 확인 (자격증명 없이)**

Run: `go run ./cmd/tesla-sentry afterblow-cancel; echo "exit=$?"`
Expected: `unknown command`가 아니라 설정/토큰 로드 에러(`load config` 등)로 종료(exit=1). 즉 라우팅은 성공.

- [ ] **Step 6: (요청 시) 커밋**

```bash
git add cmd/tesla-sentry/main.go
git commit -m "feat: afterblow-cancel 서브커맨드 추가"
```

---

## Task 5: bash `ab_parse_cancel` 파서

**Files:**
- Modify: `scripts/afterblow-lib.sh`
- Modify: `scripts/test-afterblow-parse.sh`

**Interfaces:**
- Produces: `ab_parse_cancel <line>` — 정확히 `afterblow cancel` 2토큰이면 `cancel` 출력 후 return 0, 아니면 return 1. Task 7(listener)이 사용.

- [ ] **Step 1: 실패하는 테스트 작성**

`scripts/test-afterblow-parse.sh` 의 `exit "$fail"` 직전에 추가:
```bash
check "cancel ok"        "cancel" "$(ab_parse_cancel 'afterblow cancel')"
for bad in 'afterblow' 'afterblow 2' 'afterblow cancel now' 'cancel' 'hello'; do
	if ab_parse_cancel "$bad" >/dev/null 2>&1; then
		printf 'FAIL - cancel rejected: %s\n' "$bad"; fail=1
	else
		printf 'ok   - cancel rejected: %s\n' "$bad"
	fi
done
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-afterblow-parse.sh`
Expected: `ab_parse_cancel: command not found` 류로 FAIL.

- [ ] **Step 3: `ab_parse_cancel` 구현**

`scripts/afterblow-lib.sh` 의 `ab_parse_sentry` 함수 뒤(파일 끝)에 추가:
```bash
# ab_parse_cancel <line> -> "afterblow cancel" 정확히 2토큰이면 "cancel"을 출력하고
#   return 0. 아니면 return 1. (둘째 토큰을 분으로 보는 ab_parse_message가
#   "cancel"을 3으로 오해하지 않도록, 리스너에서 이 파서를 먼저 호출한다.)
ab_parse_cancel() {
	local line="$1"
	local toks
	read -r -a toks <<<"$line"
	[ "${#toks[@]}" -eq 2 ] || return 1
	[ "${toks[0]}" = "afterblow" ] || return 1
	[ "${toks[1]}" = "cancel" ] || return 1
	printf 'cancel'
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `bash scripts/test-afterblow-parse.sh`
Expected: 모든 라인 `ok`, 종료코드 0.

- [ ] **Step 5: (요청 시) 커밋**

```bash
git add scripts/afterblow-lib.sh scripts/test-afterblow-parse.sh
git commit -m "feat: afterblow cancel 메시지 파서(ab_parse_cancel) 추가"
```

---

## Task 6: 리스너에 cancel 분기 추가

**Files:**
- Modify: `scripts/afterblow-listener.sh`
- Modify: `scripts/test-afterblow-listener.sh`

**Interfaces:**
- Consumes: `ab_parse_cancel` (Task 5), `tesla-sentry afterblow-cancel` (Task 4).
- Produces: ntfy `afterblow cancel` 수신 시 cancel 실행. `run_cancel` 함수(테스트에서 오버라이드 가능).

- [ ] **Step 1: 실패하는 테스트 작성 (cancel/sentry 스텁)**

`scripts/test-afterblow-listener.sh` 를 아래로 교체(소싱 후 `run_sentry`/`run_cancel`을 기록 스텁으로 덮어써 실제 바이너리를 부르지 않게 한다):
```bash
#!/usr/bin/env bash
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export LOG=/dev/null
# shellcheck source=afterblow-listener.sh
source "$DIR/afterblow-listener.sh"

RECORD="$(mktemp)"
record() { printf '%s\n' "$*" >>"$RECORD"; }
# 실제 바이너리 호출을 막고 분기 결과만 기록한다.
run_sentry() { printf 'sentry %s\n' "$1" >>"$RECORD"; }
run_cancel() { printf 'cancel\n' >>"$RECORD"; }

printf 'afterblow 2 vent\n\nhello world\nafterblow cancel\nsentry on\nsentry status\nafterblow 1\n' \
	| ab_consume_stream record

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

check "count"   "5"          "${#lines[@]}"
check "line0"   "2 vent"     "${lines[0]:-}"
check "line1"   "cancel"     "${lines[1]:-}"
check "line2"   "sentry on"  "${lines[2]:-}"
check "line3"   "sentry status" "${lines[3]:-}"
check "line4"   "1"          "${lines[4]:-}"

exit "$fail"
```
(주: `sentry status` 라인은 Task 9에서 파서가 status를 허용해야 통과한다. 이 태스크에서는 cancel/sentry-on/afterblow 분기까지 맞추고, status 라인 통과는 Task 9 완료 후 확정된다. Task 9 전이라면 임시로 `sentry status` 줄과 line3 체크, count를 5→4·line4 인덱스 조정으로 검증 후 Task 9에서 복원해도 된다.)

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-afterblow-listener.sh`
Expected: cancel 분기가 없어 `afterblow cancel`이 분 파서로 새거나 카운트가 어긋나 FAIL.

- [ ] **Step 3: `run_cancel` 함수 추가**

`scripts/afterblow-listener.sh` 의 `run_sentry()` 정의 바로 아래에 추가:
```bash
# afterblow 취소 실행 래퍼(공조off+창문닫기).
run_cancel() { "$ROOT/tesla-sentry" afterblow-cancel >>"$LOG" 2>&1 || log "tesla-sentry afterblow-cancel exited non-zero"; }
```

- [ ] **Step 4: `ab_consume_stream`에 cancel 분기 추가**

`scripts/afterblow-listener.sh` 의 `ab_consume_stream` 내부, `recv:` 로그 다음의 분기를 아래로 교체(분기 순서: cancel → sentry → afterblow):
```bash
		[ -z "$line" ] && continue # keepalive
		log "recv: $line"
		local sentry_arg
		if ab_parse_cancel "$line" >/dev/null; then
			log "cancel"
			run_cancel
		elif sentry_arg="$(ab_parse_sentry "$line")"; then
			log "sentry $sentry_arg"
			run_sentry "$sentry_arg"
		else
			ab_dispatch_line "$line" "$handler" || true
		fi
```

- [ ] **Step 5: 테스트 통과 확인**

Run: `bash scripts/test-afterblow-listener.sh`
Expected: (Task 9까지 끝났다면) 모든 라인 `ok`. Task 9 전이라면 Step 1 주석대로 `sentry status` 부분만 임시 처리.

- [ ] **Step 6: (요청 시) 커밋**

```bash
git add scripts/afterblow-listener.sh scripts/test-afterblow-listener.sh
git commit -m "feat: 리스너에 afterblow cancel 분기 추가"
```

---

## Task 7: webapp 취소 버튼

**Files:**
- Modify: `webapp/message.js`
- Modify: `webapp/message.test.mjs`
- Modify: `webapp/index.html`
- Modify: `webapp/app.js`

**Interfaces:**
- Consumes: 기존 `TOPIC_URL`, `setStatus(text, kind)` (app.js).
- Produces: `buildCancelMessage()` → `"afterblow cancel"`; index.html 버튼 `#cancel`.

- [ ] **Step 1: 실패하는 테스트 작성**

`webapp/message.test.mjs` 의 import에 `buildCancelMessage` 추가하고, `console.log` 직전에 단언 추가:
```javascript
import { buildMessage, clampMinutes, buildCancelMessage } from './message.js';
```
```javascript
assert.equal(buildCancelMessage(), 'afterblow cancel');
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `node webapp/message.test.mjs`
Expected: FAIL — `buildCancelMessage is not a function`.

- [ ] **Step 3: `buildCancelMessage` 구현**

`webapp/message.js` 끝에 추가:
```javascript
// 리스너가 취소(공조off+창문닫기)로 인식하는 ntfy 본문.
export function buildCancelMessage() {
  return 'afterblow cancel';
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `node webapp/message.test.mjs`
Expected: `message tests passed`.

- [ ] **Step 5: index.html에 취소 버튼 추가**

`webapp/index.html` 의
```html
    <button id="start" class="start">건조 시작</button>
```
바로 아래에 추가:
```html
    <button id="cancel" class="start cancel">취소 (공조 끄고 창문 닫기)</button>
```

- [ ] **Step 6: app.js에 취소 핸들러 추가**

`webapp/app.js` 의 import에 `buildCancelMessage` 추가:
```javascript
import { buildMessage, buildCancelMessage } from './message.js';
```
그리고 `startBtn.addEventListener('click', trigger);` 줄 바로 아래에 추가:
```javascript
const cancelBtn = document.getElementById('cancel');

async function triggerCancel() {
  cancelBtn.disabled = true;
  setStatus('취소 전송 중…', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: buildCancelMessage(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    setStatus('취소 전송됨 ✓ (공조 끄고 창문 닫기)', 'ok');
  } catch (e) {
    setStatus(`취소 전송 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    cancelBtn.disabled = false;
  }
}
cancelBtn.addEventListener('click', triggerCancel);
```

- [ ] **Step 7: 수동 확인**

`webapp/index.html` 을 브라우저로 열어 "취소" 버튼이 보이고, 클릭 시 상태 영역에 전송 결과가 표시되는지 확인(실제 ntfy 전송됨). 네트워크 탭에서 `POST .../tesla-ab-...` 본문이 `afterblow cancel`인지 확인.

- [ ] **Step 8: (요청 시) 커밋**

```bash
git add webapp/message.js webapp/message.test.mjs webapp/index.html webapp/app.js
git commit -m "feat: 폰 앱에 애프터블로우 취소 버튼 추가"
```

---

## Task 8: Worker — 상태 모델 + `validateStateInput`

**Files:**
- Modify: `worker/src/schedule.js`
- Modify: `worker/src/schedule.test.mjs`

**Interfaces:**
- Produces: `DEFAULT_SCHEDULE`에 `lastState/lastStateAt/lastStateSource` (모두 `null`); `validateStateInput(obj)` → `{ok, value:{state, source}}` 또는 `{ok:false, error}`. Task 9(index.js)·Task 13(app.js)이 사용.

- [ ] **Step 1: 실패하는 테스트 작성**

`worker/src/schedule.test.mjs` 의 import에 `validateStateInput` 추가:
```javascript
import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput, validateStateInput,
} from './schedule.js';
```
파일 끝(마지막 `console.log`가 있다면 그 직전, 없으면 끝)에 추가:
```javascript
// --- DEFAULT_SCHEDULE 상태 필드 ---
assert.equal(DEFAULT_SCHEDULE.lastState, null);
assert.equal(DEFAULT_SCHEDULE.lastStateAt, null);
assert.equal(DEFAULT_SCHEDULE.lastStateSource, null);

// --- validateStateInput ---
assert.deepEqual(validateStateInput({ state: 'on', source: 'command' }),
  { ok: true, value: { state: 'on', source: 'command' } });
assert.deepEqual(validateStateInput({ state: 'off', source: 'status' }),
  { ok: true, value: { state: 'off', source: 'status' } });
assert.equal(validateStateInput({ state: 'maybe', source: 'command' }).ok, false);
assert.equal(validateStateInput({ state: 'on', source: 'bogus' }).ok, false);
assert.equal(validateStateInput(null).ok, false);
assert.equal(validateStateInput('x').ok, false);
```
(주: 기존 schedule.test.mjs에 종료 `console.log`가 없다면 위 단언만 추가. 있다면 그 위에 둔다.)

- [ ] **Step 2: 테스트 실패 확인**

Run: `node worker/src/schedule.test.mjs`
Expected: FAIL — `validateStateInput is not a function` 또는 `lastState` 단언 실패.

- [ ] **Step 3: `DEFAULT_SCHEDULE` 확장**

`worker/src/schedule.js` 의 `DEFAULT_SCHEDULE`를:
```javascript
export const DEFAULT_SCHEDULE = {
  onTime: '05:30',
  offTime: '22:16',
  enabled: true,
  lastOn: '',
  lastOff: '',
  lastState: null,
  lastStateAt: null,
  lastStateSource: null,
};
```

- [ ] **Step 4: `validateStateInput` 구현**

`worker/src/schedule.js` 의 `validateScheduleInput` 함수 뒤(파일 끝)에 추가:
```javascript
// PC가 보낸 감시모드 상태 보고 검증. lastStateAt은 Worker가 찍는다(받지 않음).
export function validateStateInput(obj) {
  if (obj == null || typeof obj !== 'object') return { ok: false, error: 'body must be an object' };
  const { state, source } = obj;
  if (state !== 'on' && state !== 'off') return { ok: false, error: 'state must be "on" or "off"' };
  if (source !== 'command' && source !== 'status') return { ok: false, error: 'source must be "command" or "status"' };
  return { ok: true, value: { state, source } };
}
```

- [ ] **Step 5: 테스트 통과 확인**

Run: `node worker/src/schedule.test.mjs`
Expected: 에러 없이 종료(코드 0).

- [ ] **Step 6: (요청 시) 커밋**

```bash
git add worker/src/schedule.js worker/src/schedule.test.mjs
git commit -m "feat: Worker 감시모드 상태 모델·검증(validateStateInput) 추가"
```

---

## Task 9: Worker — `PUT /api/sentry-state` 라우트 + `sentry status` 파서

**Files:**
- Modify: `worker/src/index.js`
- Modify: `scripts/afterblow-lib.sh`
- Modify: `scripts/test-sentry-parse.sh`

**Interfaces:**
- Consumes: `validateStateInput` (Task 8), 기존 `readState`/`writeState`/`json`/`CORS`/`env.SENTRY_TOKEN`.
- Produces: `PUT /api/sentry-state` 엔드포인트; `ab_parse_sentry`가 `status`도 허용.

- [ ] **Step 1: import에 `validateStateInput` 추가**

`worker/src/index.js` 상단 import:
```javascript
import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput, validateStateInput,
} from './schedule.js';
```

- [ ] **Step 2: `/api/sentry-state` 라우트 추가**

`worker/src/index.js` 의 `fetch(request, env)` 안에서, 기존 404 라인
```javascript
    if (url.pathname !== '/api/sentry-schedule') return new Response('not found', { status: 404, headers: CORS });
```
을 다음으로 교체(상태 엔드포인트를 분기로 추가):
```javascript
    if (url.pathname === '/api/sentry-state') {
      if (request.method !== 'PUT') return json({ error: 'method not allowed' }, 405);
      const auth = request.headers.get('Authorization') || '';
      if (auth !== `Bearer ${env.SENTRY_TOKEN}`) return json({ error: 'unauthorized' }, 401);
      let body;
      try { body = await request.json(); } catch { return json({ error: 'invalid json' }, 400); }
      const v = validateStateInput(body);
      if (!v.ok) return json({ error: v.error }, 400);
      const state = await readState(env);
      const next = {
        ...state,
        lastState: v.value.state,
        lastStateSource: v.value.source,
        lastStateAt: new Date().toISOString(),
      };
      await writeState(env, next);
      return json({ ok: true, value: next });
    }
    if (url.pathname !== '/api/sentry-schedule') return new Response('not found', { status: 404, headers: CORS });
```

- [ ] **Step 3: Worker 단위 테스트 회귀 확인**

Run: `node worker/src/schedule.test.mjs`
Expected: 통과(라우트는 fetch 핸들러라 단위 테스트 대상 아님 — 순수 로직만 확인).

- [ ] **Step 4: `ab_parse_sentry`에 status 허용 (실패 테스트 먼저)**

`scripts/test-sentry-parse.sh` 에 `sentry off` 체크 다음 줄에 추가:
```bash
check "sentry status" "status" "$(ab_parse_sentry 'sentry status')"
```
Run: `bash scripts/test-sentry-parse.sh`
Expected: FAIL — `sentry status`가 거부됨.

- [ ] **Step 5: `ab_parse_sentry` 수정**

`scripts/afterblow-lib.sh` 의 `ab_parse_sentry` 안 case 문:
```bash
	case "${toks[1]}" in
		on | off) printf '%s' "${toks[1]}" ;;
		*) return 1 ;;
	esac
```
을:
```bash
	case "${toks[1]}" in
		on | off | status) printf '%s' "${toks[1]}" ;;
		*) return 1 ;;
	esac
```

- [ ] **Step 6: 파서 테스트 통과 확인**

Run: `bash scripts/test-sentry-parse.sh`
Expected: 모든 라인 `ok`. (리스너 `run_sentry "status"` → `tesla-sentry status` 실행으로 자연 연결됨.)

- [ ] **Step 7: 리스너 테스트 재확인 (Task 6의 status 라인 복원)**

Run: `bash scripts/test-afterblow-listener.sh`
Expected: `sentry status` 라인 포함 모든 체크 `ok` (Task 6 Step1 원본 형태).

- [ ] **Step 8: Worker 배포 (요청 시) + 수동 검증**

Worker 변경은 배포해야 적용된다(사용자 환경). 배포는 사용자 권한이므로 요청 시:
```bash
cd worker && npx wrangler deploy
```
검증(배포 후):
```bash
curl -s -X PUT https://tesla-sentry-scheduler.yhlee512.workers.dev/api/sentry-state \
  -H "Authorization: Bearer <SENTRY_TOKEN>" -H "Content-Type: application/json" \
  -d '{"state":"on","source":"status"}'
curl -s https://tesla-sentry-scheduler.yhlee512.workers.dev/api/sentry-schedule
```
Expected: 첫 호출 `{"ok":true,...}`, 둘째 호출 JSON에 `lastState:"on"`, `lastStateSource:"status"`, `lastStateAt` 포함.

- [ ] **Step 9: (요청 시) 커밋**

```bash
git add worker/src/index.js scripts/afterblow-lib.sh scripts/test-sentry-parse.sh
git commit -m "feat: Worker 상태 보고 엔드포인트 + sentry status 메시지 파서"
```

---

## Task 10: Go config — 상태 보고 설정 필드

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `Config.SentryStateURL`(`sentry_state_url`), `Config.SentryStateToken`(`sentry_state_token`). Task 12(main.go)·Task 11(report)이 사용.

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/config/config_test.go` 에 테스트 함수 추가(기존 테스트 헬퍼 패턴 확인 후; 독립적인 round-trip 테스트):
```go
func TestConfigStateFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c := &Config{
		ClientID: "id", ClientSecret: "sec", VIN: "VIN", Domain: "d", Region: "na",
		SentryStateURL: "https://w.example/api/sentry-state", SentryStateToken: "tok",
	}
	if err := c.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.SentryStateURL != c.SentryStateURL || got.SentryStateToken != c.SentryStateToken {
		t.Fatalf("state fields not round-tripped: %+v", got)
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `go test ./internal/config/ -run TestConfigStateFieldsRoundTrip -v`
Expected: FAIL — `unknown field SentryStateURL`.

- [ ] **Step 3: Config 구조체에 필드 추가**

`internal/config/config.go` 의 `Config` 구조체:
```go
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	VIN          string `json:"vin"`
	Domain       string `json:"domain"`
	Region       string `json:"region"`
}
```
을:
```go
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	VIN          string `json:"vin"`
	Domain       string `json:"domain"`
	Region       string `json:"region"`
	// 감시모드 상태를 Worker KV에 보고할 엔드포인트/토큰. 둘 중 하나라도 비면 보고 생략.
	SentryStateURL   string `json:"sentry_state_url,omitempty"`
	SentryStateToken string `json:"sentry_state_token,omitempty"`
}
```

- [ ] **Step 4: 테스트 통과 + 회귀 확인**

Run: `go test ./internal/config/`
Expected: ok (기존 테스트 포함 통과 — 새 필드는 omitempty라 기존 config.json 호환).

- [ ] **Step 5: (요청 시) 커밋**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: config에 감시모드 상태 보고 URL/토큰 필드 추가"
```

---

## Task 11: Go `ReportSentryState` (HTTP PUT)

**Files:**
- Create: `internal/tesla/report.go`
- Test: `internal/tesla/report_test.go` (Create)

**Interfaces:**
- Consumes: 기존 `HTTPClient` (partner.go).
- Produces: `func ReportSentryState(ctx context.Context, url, token, state, source string) error` — Task 12(main.go)가 호출. `{ "state":..., "source":... }` JSON 본문을 `Authorization: Bearer <token>`으로 PUT.

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/tesla/report_test.go` 생성:
```go
package tesla

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReportSentryStatePutsBody(t *testing.T) {
	var gotMethod, gotAuth, gotState, gotSource string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		var body struct{ State, Source string }
		_ = json.Unmarshal(b, &body)
		gotState, gotSource = body.State, body.Source
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	if err := ReportSentryState(context.Background(), srv.URL, "TOK", "on", "command"); err != nil {
		t.Fatalf("ReportSentryState: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %q, want PUT", gotMethod)
	}
	if gotAuth != "Bearer TOK" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotState != "on" || gotSource != "command" {
		t.Errorf("body state/source = %q/%q", gotState, gotSource)
	}
}

func TestReportSentryStateErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()
	if err := ReportSentryState(context.Background(), srv.URL, "BAD", "off", "status"); err == nil {
		t.Fatal("expected error on 401")
	}
}
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `go test ./internal/tesla/ -run TestReportSentryState -v`
Expected: FAIL — `undefined: ReportSentryState`.

- [ ] **Step 3: `ReportSentryState` 구현**

`internal/tesla/report.go` 생성:
```go
package tesla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ReportSentryState는 감시모드 상태를 Worker(KV)에 PUT으로 보고한다. state는
// "on"/"off", source는 "command"/"status". 호출측은 best-effort로 다뤄야 하며
// (실패해도 명령 자체는 성공), url/token이 비면 호출하지 않는다.
func ReportSentryState(ctx context.Context, url, token, state, source string) error {
	payload, err := json.Marshal(map[string]string{"state": state, "source": source})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("report state %s: %s", resp.Status, string(body))
	}
	return nil
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `go test ./internal/tesla/ -run TestReportSentryState -v`
Expected: PASS (두 테스트).

- [ ] **Step 5: 전체 Go 테스트 회귀**

Run: `go test ./...`
Expected: 모든 패키지 ok.

- [ ] **Step 6: (요청 시) 커밋**

```bash
git add internal/tesla/report.go internal/tesla/report_test.go
git commit -m "feat: 감시모드 상태를 Worker에 보고하는 ReportSentryState 추가"
```

---

## Task 12: main.go — on/off/status 성공 후 상태 보고

**Files:**
- Modify: `cmd/tesla-sentry/main.go`

**Interfaces:**
- Consumes: `tesla.ReportSentryState(...)` (Task 11), `Config.SentryStateURL/Token` (Task 10), 기존 `cmdSet`/`cmdStatus`.
- Produces: 없음(부수효과: on/off/status 성공 시 KV 보고).

- [ ] **Step 1: 보고 헬퍼 추가**

`cmd/tesla-sentry/main.go` 의 `loadForCommand` 함수 뒤에 추가:
```go
// reportSentryState는 감시모드 상태를 best-effort로 Worker에 보고한다.
// URL/토큰 미설정이면 조용히 생략하고, 실패해도 명령 결과에 영향을 주지 않는다.
func reportSentryState(ctx context.Context, cfg *config.Config, state, source string) {
	if cfg.SentryStateURL == "" || cfg.SentryStateToken == "" {
		return
	}
	if err := tesla.ReportSentryState(ctx, cfg.SentryStateURL, cfg.SentryStateToken, state, source); err != nil {
		log.Printf("report sentry state (%s/%s): %v", state, source, err)
	}
}
```

- [ ] **Step 2: cmdSet에서 보고 호출**

`cmd/tesla-sentry/main.go` 의 `cmdSet` 끝부분:
```go
	log.Printf("sentry mode set to %s", state)
	return nil
}
```
을:
```go
	log.Printf("sentry mode set to %s", state)
	reportSentryState(ctx, cfg, state, "command")
	return nil
}
```

- [ ] **Step 3: cmdStatus에서 보고 호출**

`cmd/tesla-sentry/main.go` 의 `cmdStatus` 끝부분:
```go
	on, err := tesla.SentryState(ctx, at, cfg.VIN)
	if err != nil {
		return err
	}
	fmt.Printf("sentry mode: %v\n", on)
	return nil
}
```
을:
```go
	on, err := tesla.SentryState(ctx, at, cfg.VIN)
	if err != nil {
		return err
	}
	fmt.Printf("sentry mode: %v\n", on)
	state := "off"
	if on {
		state = "on"
	}
	reportSentryState(ctx, cfg, state, "status")
	return nil
}
```

- [ ] **Step 4: 빌드/베팅/테스트**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: 에러 없음, 모든 테스트 통과.

- [ ] **Step 5: (요청 시) 커밋**

```bash
git add cmd/tesla-sentry/main.go
git commit -m "feat: on/off/status 성공 시 감시모드 상태를 KV에 보고"
```

---

## Task 13: webapp — 상태 표시 + 실시간 확인 버튼

**Files:**
- Modify: `webapp/sentry.js`
- Modify: `webapp/sentry.test.mjs`
- Modify: `webapp/index.html`
- Modify: `webapp/app.js`

**Interfaces:**
- Consumes: 기존 `SENTRY_API`, `TOPIC_URL`, `setSentryStatus`, `loadSchedule`, `SENTRY_CACHE_KEY` (app.js).
- Produces: `buildSentryStatusMessage()` → `"sentry status"`; index.html `#sentryState`, `#sentryCheck`.

- [ ] **Step 1: 실패하는 테스트 작성**

`webapp/sentry.test.mjs` 의 import에 `buildSentryStatusMessage` 추가:
```javascript
import { isValidHHMM, buildSchedulePayload, buildSentryStatusMessage } from './sentry.js';
```
`console.log('sentry tests passed');` 직전에 추가:
```javascript
assert.equal(buildSentryStatusMessage(), 'sentry status');
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `node webapp/sentry.test.mjs`
Expected: FAIL — `buildSentryStatusMessage is not a function`.

- [ ] **Step 3: `buildSentryStatusMessage` 구현**

`webapp/sentry.js` 끝에 추가:
```javascript
// 리스너가 실시간 상태 조회로 인식하는 ntfy 본문.
export function buildSentryStatusMessage() {
  return 'sentry status';
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `node webapp/sentry.test.mjs`
Expected: `sentry tests passed`.

- [ ] **Step 5: index.html에 상태 표시 + 버튼 추가**

`webapp/index.html` 의 감시모드 섹션, `설정 저장` 버튼과 상태줄:
```html
    <button id="sentrySave" class="start">설정 저장</button>
    <p id="sentryStatus" class="status"></p>
```
을:
```html
    <button id="sentrySave" class="start">설정 저장</button>
    <p id="sentryStatus" class="status"></p>

    <section class="row">
      <span class="label">현재 상태</span>
      <span id="sentryState" class="value">—</span>
    </section>
    <button id="sentryCheck" class="start">실시간 확인</button>
```

- [ ] **Step 6: app.js — import + 상태 렌더 + 로드 시 표시**

`webapp/app.js` 의 import:
```javascript
import { isValidHHMM, buildSchedulePayload } from './sentry.js';
```
을:
```javascript
import { isValidHHMM, buildSchedulePayload, buildSentryStatusMessage } from './sentry.js';
```
그리고 `function applySchedule(s) { ... }` 함수 뒤에 상태 렌더 함수 추가:
```javascript
const sentryStateEl = document.getElementById('sentryState');

function renderSentryState(s) {
  if (!s || !s.lastState) {
    sentryStateEl.textContent = '상태 미상';
    return;
  }
  const label = s.lastState === 'on' ? 'ON' : 'OFF';
  const kind = s.lastStateSource === 'status' ? '실시간' : '마지막 명령';
  sentryStateEl.textContent = `${label} (${kind})`;
}
```
`loadSchedule`의 성공 분기에서 `applySchedule(s);` 다음 줄에 `renderSentryState(s);` 추가, catch 분기에서 `if (cached) applySchedule(JSON.parse(cached));` 다음 줄에 `if (cached) renderSentryState(JSON.parse(cached));` 추가:
```javascript
async function loadSchedule() {
  try {
    const res = await fetch(SENTRY_API);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const s = await res.json();
    applySchedule(s);
    renderSentryState(s);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
  } catch (e) {
    const cached = localStorage.getItem(SENTRY_CACHE_KEY);
    if (cached) { applySchedule(JSON.parse(cached)); renderSentryState(JSON.parse(cached)); }
    setSentryStatus(`현재 설정을 못 불러왔습니다 (${e.message}) — 캐시 표시`, 'err');
  }
}
```

- [ ] **Step 7: app.js — 실시간 확인 핸들러 추가**

`webapp/app.js` 의 `sentrySave.addEventListener('click', saveSchedule);` 줄 바로 아래(그리고 `loadSchedule();` 호출 위)에 추가:
```javascript
const sentryCheckBtn = document.getElementById('sentryCheck');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function checkSentryRealtime() {
  sentryCheckBtn.disabled = true;
  // 기준 타임스탬프(이 값이 바뀌면 새 조회 결과가 들어온 것).
  let baseline = null;
  try {
    const r0 = await fetch(SENTRY_API);
    if (r0.ok) baseline = (await r0.json()).lastStateAt || null;
  } catch { /* 무시: 폴링에서 다시 시도 */ }

  setSentryStatus('실시간 확인 중… (차량을 깨우지 않음 · 오프라인이면 실패)', 'pending');
  try {
    const res = await fetch(TOPIC_URL, {
      method: 'POST', headers: { 'Content-Type': 'text/plain' }, body: buildSentryStatusMessage(),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
  } catch (e) {
    setSentryStatus(`확인 요청 실패: ${e.message}`, 'err');
    sentryCheckBtn.disabled = false;
    return;
  }

  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    await sleep(2000);
    try {
      const r = await fetch(SENTRY_API);
      if (!r.ok) continue;
      const s = await r.json();
      if (s.lastStateAt && s.lastStateAt !== baseline && s.lastStateSource === 'status') {
        renderSentryState(s);
        localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
        setSentryStatus('실시간 확인 완료 ✓', 'ok');
        sentryCheckBtn.disabled = false;
        return;
      }
    } catch { /* 무시: 다음 폴링 */ }
  }
  setSentryStatus('확인 실패 — 차량이 오프라인이거나 PC가 꺼져 있을 수 있음', 'err');
  sentryCheckBtn.disabled = false;
}
sentryCheckBtn.addEventListener('click', checkSentryRealtime);
```

- [ ] **Step 8: webapp 단위 테스트 회귀**

Run: `node webapp/message.test.mjs && node webapp/sentry.test.mjs`
Expected: 둘 다 통과 메시지.

- [ ] **Step 9: 수동 확인 (엔드투엔드)**

전제: Task 9 Worker 배포 완료, PC에서 리스너 실행 중, config.json에 `sentry_state_url`/`sentry_state_token` 설정됨.
1. `webapp/index.html` 로드 → "현재 상태"가 마지막 명령 기준으로 표시(`ON (마지막 명령)` 등) 또는 `상태 미상`.
2. "실시간 확인" 클릭 → "실시간 확인 중…" → 차량 online이면 수 초~30초 내 `ON/OFF (실시간)` + "완료 ✓". 차량 asleep이면 30초 후 "확인 실패 — …오프라인…".

- [ ] **Step 10: (요청 시) 커밋**

```bash
git add webapp/sentry.js webapp/sentry.test.mjs webapp/index.html webapp/app.js
git commit -m "feat: 폰 앱에 감시모드 상태 표시 + 실시간 확인 버튼 추가"
```

---

## 최종 검증 (전체)

- [ ] **Go**: `go build ./... && go vet ./... && go test ./...` — 모두 통과
- [ ] **bash 테스트**: `for t in scripts/test-*.sh; do echo "== $t =="; bash "$t"; done` — 모두 `ok`
- [ ] **JS 테스트**: `node webapp/message.test.mjs && node webapp/sentry.test.mjs && node worker/src/schedule.test.mjs` — 모두 통과
- [ ] **로그 통합 확인**: afterblow/sentry 동작을 한 번씩 트리거한 뒤 `프로젝트루트/tesla.log`에 `[run]`/`[listener]`/`[sched]` 태그가 함께 쌓이는지 확인. `~/.config/tesla-sentry/sentry.log`에 새 줄이 더는 추가되지 않는지 확인.
- [ ] **CSS (선택)**: `webapp/style.css`에 `.cancel`(취소 버튼 색 구분) 스타일을 둘지 검토 — 없어도 `.start` 기본 스타일로 동작하므로 필수는 아님.

---

## Self-Review 결과 (작성자 점검)

- **스펙 커버리지**: 기능1=Task1 / 기능2=Task3~7 / 기능3=Task2 / 기능4=Task8~13. 스펙의 모든 섹션 대응됨.
- **타입/이름 일관성**: `ReportSentryState(ctx,url,token,state,source)`·`validateStateInput`·`buildCancelMessage`·`buildSentryStatusMessage`·`ab_parse_cancel`·`AfterBlowCancel`·`lastState/lastStateAt/lastStateSource`·`sentry_state_url/sentry_state_token` 모두 태스크 간 동일 표기 확인.
- **상태 source 값**: `"command"`/`"status"` 2종으로 Worker 검증·Go 보고·UI 라벨 전부 일치.
- **순서 의존성**: Task6의 `sentry status` 리스너 케이스는 Task9(파서 status 허용) 완료 후 최종 통과 — Task6 Step1에 임시 처리 안내 명시. Task13의 E2E는 Task9 배포 전제 명시.
- **플레이스홀더 스캔**: TBD/TODO 없음. 모든 코드 스텝에 실제 코드 포함.
