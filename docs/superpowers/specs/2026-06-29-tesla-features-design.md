# 설계: 로그 통합 · 애프터블로우 취소 · 기본값 1분 · 감시모드 상태 표시

> 작성일: 2026-06-29
> 범위: 4개의 독립 기능을 한 스펙으로 묶음. 각 기능은 서로 의존하지 않으므로 순서 무관하게 구현 가능하나, 기능 4가 가장 큼.

---

## 배경 (현재 구조)

- **Go 바이너리 `tesla-sentry`**: `cmd/tesla-sentry/main.go` + `internal/` (config·oauth·tesla·keys). `on`/`off`/`status`/`afterblow` 서브커맨드. 설정·토큰·키는 `~/.config/tesla-sentry/`(또는 `XDG_CONFIG_HOME`)에 보관.
- **애프터블로우 경로**: 폰 PWA → ntfy 발행 → `scripts/afterblow-listener.sh`(상시 구독) → `scripts/afterblow-run.sh` → `tesla-sentry afterblow N [vent]`. 이 명령은 한 프로세스가 "예열 시작 → N분 대기 → 종료(공조off+창문닫기)"를 **동기 수행**.
- **감시모드 경로**: 폰 PWA → Cloudflare Worker(KV) → PC crontab(`scripts/sentry-schedule-check.sh`, 매분) → `tesla-sentry on/off`. ntfy로 `sentry on/off`도 수신(`afterblow-listener.sh`).
- **현재 로그 위치**: 애프터블로우 = `프로젝트루트/afterblow.log`, 감시모드 스케줄 = `~/.config/tesla-sentry/sentry.log` (Go 출력은 스크립트가 각 로그로 `>> 2>&1` 리다이렉트).

---

## 기능 1 — 로그 통합 (`프로젝트루트/tesla.log` 단일 파일)

### 결정
모든 로그를 프로젝트 루트의 **단일 파일 `tesla.log`** 로 모은다. 각 스크립트의 `log()`가 이미 `[run]`/`[listener]`/`[sched]` 태그를 붙이므로 한 파일에서 출처를 구분할 수 있다. 설정·토큰·키·락 파일 등 **로그가 아닌 런타임/민감 파일은 `~/.config`에 그대로 둔다**(로그만 이동).

### 변경
- `scripts/afterblow-run.sh`: `LOG` 기본값 `$ROOT/afterblow.log` → `$ROOT/tesla.log`
- `scripts/afterblow-listener.sh`: `LOG` 기본값 `$ROOT/afterblow.log` → `$ROOT/tesla.log`
- `scripts/sentry-schedule-check.sh`: `LOG` 기본값 `$STATE_DIR/sentry.log` → `$ROOT/tesla.log`. `STATE_DIR`(락 파일 `.sentry-check.lock` 위치)은 그대로 유지.
- `.gitignore`: `afterblow.log` 항목 → `tesla.log` 로 교체
- `README.md`: `sentry.log`/로그 위치 설명을 `tesla.log`로 갱신

### 동시성/에러
- 두 자동화가 동시에 한 줄씩 append 하는 경우: 리눅스에서 `O_APPEND` + 한 줄(<4KB) write는 사실상 원자적이라 줄 섞임 위험이 낮다. Go 바이너리의 다줄 출력이 드물게 끼어들 수 있으나 허용 범위.
- 각 스크립트는 `LOG` 환경변수로 경로를 덮어쓸 수 있어 테스트는 영향 없음(기존 테스트가 임시 LOG 주입).

### 테스트
- 기존 스크립트 테스트가 `LOG` 주입으로 통과하는지 확인(경로 기본값 변경만이므로 회귀 없음).

---

## 기능 2 — 애프터블로우 취소 버튼 (공조 off + 창문 닫기)

### 결정
"취소"는 **실행 중인 afterblow 프로세스를 죽이지 않는 독립 명령**이다. 별도로 공조off + 창문닫기를 차량에 보낸다.

근거: afterblow는 한 프로세스가 동기적으로 "예열 → N분 대기 → 종료(공조off+창문닫기)"를 수행한다. 취소가 독립 명령이면, 남아 돌던 프로세스가 N분 뒤 종료 단계를 또 실행해도 *이미 꺼진 공조를 다시 끄고 닫힌 창문을 다시 닫는* 무해한 재실행이다. 반면 프로세스 pkill 방식은 pidfile·패턴매칭이 필요하고, 그 사이 새로 시작된 afterblow를 잘못 죽일 위험이 있어 채택하지 않는다.

### 변경
- **Go**
  - `cmd/tesla-sentry/main.go`: switch에 `case "afterblow-cancel"` 추가. usage 문자열에 반영.
  - `internal/tesla/command.go`: 새 함수 `AfterBlowCancel(ctx, accessToken, vin, privateKeyPath)` → `withVehicle` 안에서 `ClimateOff` + `CloseWindows`를 best-effort(`errors.Join`)로 실행. 차가 자고 있으면 깨워서 보낸다(취소엔 깨움 불가피). 창문은 vent 여부와 무관하게 무조건 닫는다(이미 닫혀 있어도 무해).
- **ntfy 프로토콜 / 스크립트**
  - ⚠️ 현재 `ab_parse_message`는 둘째 토큰을 분으로 보며, `"cancel"`은 비숫자라 기본 3으로 정규화되어 *3분 afterblow가 잘못 실행*된다. 따라서 cancel을 **먼저 가로채야** 한다.
  - `scripts/afterblow-lib.sh`: `ab_parse_cancel <line>` 추가 — 정확히 `afterblow cancel` 2토큰일 때만 매치(`ab_parse_sentry`와 동일 패턴, 그 외 return 1).
  - `scripts/afterblow-listener.sh` `ab_consume_stream`: 분기 순서를 **cancel → sentry → afterblow(일반)** 로. cancel 매치 시 `run_cancel`이 `$ROOT/tesla-sentry afterblow-cancel` 실행(로그 리다이렉트, 비정상 종료 로깅).
- **webapp**
  - `webapp/index.html`: "건조 시작" 버튼 아래에 **"취소 (공조 끄고 창문 닫기)"** 버튼 추가. 앱은 afterblow 실행 여부를 모르므로 항상 표시되는 독립 액션으로 둔다.
  - `webapp/message.js`: `buildCancelMessage()` → 문자열 `"afterblow cancel"` 반환(순수 함수).
  - `webapp/app.js`: 취소 버튼 클릭 → ntfy POST, 기존 `setStatus` 패턴으로 전송 결과 표시(전송 성공 ≠ 차량 동작 확인 주석 유지).

### 에러
- Go 종료 단계는 best-effort로 모든 단계를 시도(`errors.Join`)하여 일부 실패해도 나머지를 수행.
- 남은 afterblow 프로세스의 후속 종료 단계는 무해한 재실행(설계상 허용).

### 테스트
- `scripts/test-afterblow-parse.sh` 류: `ab_parse_cancel` 매치/비매치, `"afterblow cancel"`이 분 파서로 새지 않는지.
- `scripts/test-afterblow-listener.sh`: cancel 분기가 cancel 핸들러를 호출하는지.
- `webapp/message.test.mjs`: `buildCancelMessage`.

---

## 기능 3 — 애프터블로우 폰 기본값 1분

### 결정
폰 PWA를 열면 건조 시간 슬라이더가 **1분**에 가 있어야 한다. 서버측 방어 폴백은 유지.

### 변경
- `webapp/index.html`: 슬라이더 `value="3"` → `value="1"`, 초기 라벨 `<span id="minutesLabel">3분</span>` → `1분`. (min=1, max=3, step=1 유지)
- 폴백 값은 변경하지 않음: `webapp/message.js`의 `clampMinutes` 기본(3)·bash `ab_sanitize_minutes` 기본(3)은 *잘못된 입력에 대한 방어값*이며, 폰은 항상 1~3 유효값을 보내므로 실제로 타지 않는다.

### 테스트
- 별도 테스트 불필요(정적 기본값 변경). 필요 시 `app.js`의 초기 라벨 렌더가 슬라이더 값을 따르는지만 수동 확인.

---

## 기능 4 — 감시모드 on/off 상태 표시 (기본=마지막 명령, 버튼=실시간)

폰 앱에 두 가지를 표시한다: 평소엔 **KV의 "마지막 명령 상태"**(무료, 깨움 0), "실시간 확인" 버튼을 누르면 **실제 차량 상태 1회 조회**.

### 데이터 모델 (Worker KV 확장)
기존 schedule 상태(`onTime/offTime/enabled/lastOn/lastOff/updatedAt`)에 필드 추가:
- `lastState`: `"on" | "off" | null` (null = 미상)
- `lastStateAt`: ISO 시각 문자열
- `lastStateSource`: `"command" | "status"` (스케줄/명령으로 설정됨 vs 실시간 조회)

`worker/src/schedule.js`의 `DEFAULT_SCHEDULE`에 위 기본값(`null`/`null`/`null`) 추가 → 기존 `GET /api/sentry-schedule`이 자동으로 함께 반환.

### 상태 보고 주체 — Go 바이너리가 직접 KV에 보고
PC 스크립트(cron·ntfy)를 개별 수정하는 대신, `tesla-sentry` 바이너리가 명령 성공 후 KV에 보고한다. cron·ntfy 두 경로가 모두 결국 `tesla-sentry on/off`를 호출하므로 보고 지점이 한 곳(Go)으로 모인다.

- `internal/config/config.go`: `Config`에 선택적 필드 `SentryStateURL`(`sentry_state_url`), `SentryStateToken`(`sentry_state_token`) 추가. **둘 중 하나라도 비어 있으면 보고 생략** → 기존 환경·테스트와 분리.
- `internal/tesla/`(예: 신규 `state_report.go` 또는 `command.go` 확장): `ReportSentryState(ctx, url, token, state, source)` → Worker에 `PUT` (HTTP). 네트워크/non-2xx는 에러 반환.
- `cmd/tesla-sentry/main.go`:
  - `cmdSetSentry`(on/off) 성공 후 → `ReportSentryState(..., state, "command")` 호출
  - `cmdStatus` 성공(실제 상태 읽음) 후 → `ReportSentryState(..., state, "status")` 호출
  - **모든 보고는 best-effort**: 실패해도 명령 자체는 성공 처리하고 `log.Printf`로만 남긴다(`SentryStateURL` 미설정이면 조용히 생략).

### Worker — 상태 기록 엔드포인트
- `worker/src/index.js`: 새 라우트 `PUT /api/sentry-state` (기존 `PUT /api/sentry-schedule`는 schedule 입력을 엄격 검증하므로 분리). `Authorization: Bearer <SENTRY_TOKEN>` 인증.
- 본문 `{ state: "on"|"off", source: "command"|"status" }` 검증(`schedule.js`에 `validateStateInput` 추가) 후 KV의 `lastState/lastStateAt/lastStateSource` 갱신(`lastStateAt`은 Worker에서 `new Date().toISOString()`).
- CORS `Access-Control-Allow-Methods`에 변화 없음(PUT 이미 허용). 라우트 매칭만 추가.

### 실시간 확인 버튼 흐름 (비동기 라운드트립)
```
폰 [실시간 확인] 클릭
  → ntfy 발행 "sentry status"
  → PC 리스너가 받아 tesla-sentry status 실행
  → status가 실제 차량 상태를 KV에 PUT (source:"status", lastStateAt 갱신)
  → 폰이 Worker GET을 폴링(2초 간격, 최대 ~30초)하며 lastStateAt 변화 감지
  → 갱신되면 "감시 모드: ON/OFF (실시간)" 표시
  → 타임아웃이면 실패 안내, 마지막 명령 기준 표시 유지
```
- `scripts/afterblow-listener.sh`: `ab_parse_sentry`를 `on|off|status` 수용으로 확장(또는 `status` 분기 추가). `sentry status` 메시지면 `tesla-sentry status` 실행.
- **실시간 확인은 차를 강제로 깨우지 않는다**(배터리 철학 유지). `status`는 `vehicle_data`를 읽는데 차가 online일 때만 응답한다. 차가 자고 있으면 응답이 없어 폴링이 타임아웃 → "오프라인" 안내로 처리한다.

### 폰 UI (감시 모드 섹션)
- `webapp/index.html`: 감시 모드 섹션에 상태 표시 영역 + "실시간 확인" 버튼 추가.
- `webapp/app.js`:
  - 페이지 로드 시 GET으로 `lastState` 표시. 예) `감시 모드: ON · 마지막 명령 22:00`(`lastStateSource`에 따라 문구 보강). `lastState`가 null이면 `상태 미상`.
  - 실시간 버튼 → ntfy 발행 후 GET 폴링(로딩 표시), 갱신 감지 시 `(실시간)` 라벨로 표시, 타임아웃 시 실패 안내.
  - 네트워크 실패 시 기존 `localStorage` 캐시 표시 패턴 재사용.
- `webapp/message.js`(또는 `sentry.js`): `buildSentryStatusMessage()` → `"sentry status"`.

### 구성요소 요약
| 영역 | 변경 |
|---|---|
| `worker/src/schedule.js` | `DEFAULT_SCHEDULE`에 lastState 필드, `validateStateInput` |
| `worker/src/index.js` | `PUT /api/sentry-state` 라우트 |
| `internal/config/config.go` | `Config`에 `sentry_state_url`/`sentry_state_token` |
| `internal/tesla/` | `ReportSentryState` (HTTP PUT) |
| `cmd/tesla-sentry/main.go` | on/off/status 성공 후 보고 호출(best-effort) |
| `scripts/afterblow-listener.sh` | `sentry status` 분기 |
| `webapp/index.html` | 상태 표시 + 실시간 버튼 |
| `webapp/app.js` | 로드시 표시, 버튼→발행+폴링 |
| `webapp/message.js`(또는 `sentry.js`) | `buildSentryStatusMessage()` |

### 에러 처리
- Go 상태 보고 실패는 명령을 깨지 않음(로그만, URL 미설정 시 생략).
- 실시간 조회: 차 오프라인/PC 오프 시 폴링 타임아웃 → 안내, 마지막 명령 표시 유지.
- 폰 네트워크 실패: localStorage 캐시 표시.

### 테스트
- `worker/src/schedule.test.mjs`: `DEFAULT_SCHEDULE` 새 필드, `validateStateInput` 정상/이상 입력.
- Go: `ReportSentryState` httptest(정상 PUT/인증헤더/non-2xx), `cmdStatus`·on/off가 보고를 호출하는지(또는 URL 미설정 시 생략).
- `scripts/test-sentry-parse.sh`: `sentry status` 파싱.
- `webapp/sentry.test.mjs` / `message.test.mjs`: `buildSentryStatusMessage`.

---

## 구현 순서 제안
1. 기능 1(로그) — 가장 단순, 회귀 위험 낮음
2. 기능 3(기본값 1분) — 정적 변경
3. 기능 2(취소) — Go+스크립트+webapp, 독립적
4. 기능 4(상태 표시) — 가장 큼(Worker+Go+스크립트+webapp)

각 기능은 서로 의존하지 않으므로 부분 구현·검증 가능.
