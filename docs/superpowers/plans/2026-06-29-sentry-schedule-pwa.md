# 감시 모드(Sentry) 스케줄 PWA 제어 — 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 폰(PWA)에서 Sentry ON/OFF 시각과 마스터 토글을 설정하면 Cloudflare KV에 저장되고, Worker Cron이 매 분 판정해 ntfy로 신호를 보내 PC가 `tesla-sentry on/off`를 실행한다.

**Architecture:** 설정의 단일 진실은 Cloudflare KV. Cloudflare Worker가 (a) 폰의 GET/PUT을 받는 API와 (b) 1분 주기 Cron 판정을 겸한다. Cron이 KST 기준으로 시각 도달 시 ntfy 공유 토픽에 `sentry on`/`sentry off`를 발행하고, 기존 PC bash 리스너가 이를 분기해 `tesla-sentry on/off`를 호출한다. PC의 crontab sentry 줄은 제거한다.

**Tech Stack:** Cloudflare Workers + KV (ES modules), Vanilla JS PWA, Bash, ntfy.sh. 테스트는 `node:assert/strict`(JS)와 source 후 `check` 비교(bash) — 기존 프로젝트 패턴.

설계 문서: `docs/superpowers/specs/2026-06-29-sentry-schedule-pwa-design.md`

## Global Constraints

- **시간대**: 모든 `onTime`/`offTime`은 `HH:MM` 24시간제, **KST(Asia/Seoul, UTC+9) 고정**. Worker는 UTC로 동작하므로 내부에서 +9h 변환 후 비교한다.
- **시각 비교는 zero-padded `HH:MM` 문자열 사전순**으로 한다(`"05:30" < "22:16"`). 검증이 항상 2자리 zero-pad를 보장하므로 사전순 == 시간순이다.
- **신뢰 불가 입력**: 공개 URL이므로 Worker PUT은 입력을 항상 재검증한다. PC bash도 `sentry` 뒤 토큰은 `on|off`만 허용한다.
- **`enabled=false`는 자동화 정지**(ON/OFF 둘 다 발행 안 함)이며 강제 OFF가 아니다.
- **KV 쓰기는 발행 순간에만**(하루 약 2회) — Cron은 평소 읽기만 하고, 조건 충족 시에만 `KV.put` 한다(무료 한도·과금 보호).
- **ntfy 토픽**: 애프터블로우와 동일한 `https://ntfy.sh/tesla-ab-9f3k7q2zx8m`. 메시지 첫 토큰으로 분기.
- **커밋 정책**: 프로젝트 `CLAUDE.md`에 따라 **커밋은 사용자가 "커밋 & 푸시해줘"라고 명시 요청할 때만** 수행한다. 아래 각 Task의 "커밋" step은 그 시점에 사용자가 승인하면 실행하고, 아니면 변경만 유지한 채 다음 Task로 진행한다(커밋 메시지는 그대로 사용).
- **배포 산출 값(불가피한 환경값)**: Worker의 `*.workers.dev` URL과 PUT 시크릿 토큰은 배포 시 정해진다. Task 2에서 Worker를 먼저 배포·확정하고, 그 값을 Task 4의 PWA 상수에 기입한다.

---

## 파일 구조

신규/수정 파일과 책임:

| 파일 | 책임 |
|---|---|
| `worker/src/schedule.js` (신규) | 순수 로직: KST 변환, 발행 판정, 입력 검증, 기본값. I/O 없음 → 테스트 대상 |
| `worker/src/schedule.test.mjs` (신규) | `schedule.js` 단위 테스트 |
| `worker/src/index.js` (신규) | Worker 엔트리: `fetch`(GET/PUT) + `scheduled`(Cron). KV·ntfy I/O는 여기서만 |
| `worker/wrangler.toml` (신규) | Worker 설정: KV 바인딩, cron 트리거, 변수 |
| `worker/README.md` (신규) | Worker 배포·시크릿 주입 절차 |
| `webapp/sentry.js` (신규) | 순수 로직: 시각 검증, PUT 페이로드 빌드 → 테스트 대상 |
| `webapp/sentry.test.mjs` (신규) | `sentry.js` 단위 테스트 |
| `webapp/app.js` (수정) | Sentry 섹션 로드(GET)/저장(PUT) 로직 추가 |
| `webapp/index.html` (수정) | Sentry 섹션 마크업 추가 |
| `webapp/style.css` (수정) | Sentry 섹션 스타일(시각 입력·구분선) |
| `scripts/afterblow-lib.sh` (수정) | `ab_parse_sentry` 순수 파서 추가 |
| `scripts/test-sentry-parse.sh` (신규) | `ab_parse_sentry` 단위 테스트 |
| `scripts/afterblow-listener.sh` (수정) | 스트림 분기에 sentry 처리 추가 |
| `docs/superpowers/specs/2026-06-29-sentry-schedule-pwa-design.md` 의 crontab 정리 | Task 6에서 crontab 줄 제거 안내 |

---

## Task 1: Worker 스케줄 순수 로직 (`schedule.js`)

**Files:**
- Create: `worker/src/schedule.js`
- Test: `worker/src/schedule.test.mjs`

**Interfaces:**
- Consumes: 없음.
- Produces (Task 2가 사용):
  - `DEFAULT_SCHEDULE` — `{ onTime: "05:30", offTime: "22:16", enabled: true, lastOn: "", lastOff: "" }`
  - `kstParts(epochMs: number)` → `{ today: "YYYY-MM-DD", hhmm: "HH:MM" }` (KST)
  - `decideActions(state, nowParts)` → `{ fireOn: boolean, fireOff: boolean }`
    - `state`: `{ onTime, offTime, enabled, lastOn, lastOff }`
    - `nowParts`: `{ today, hhmm }`
  - `validateScheduleInput(obj)` → `{ ok: true, value: { onTime, offTime, enabled } }` 또는 `{ ok: false, error: string }`

- [ ] **Step 1: 실패하는 테스트 작성**

Create `worker/src/schedule.test.mjs`:

```javascript
import assert from 'node:assert/strict';
import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput,
} from './schedule.js';

// --- kstParts: UTC+9 변환 ---
// 2026-06-29T00:00:00Z = KST 2026-06-29 09:00
{
  const p = kstParts(Date.UTC(2026, 5, 29, 0, 0, 0));
  assert.equal(p.today, '2026-06-29');
  assert.equal(p.hhmm, '09:00');
}
// 2026-06-29T20:00:00Z = KST 2026-06-30 05:00 (날짜 넘어감)
{
  const p = kstParts(Date.UTC(2026, 5, 29, 20, 0, 0));
  assert.equal(p.today, '2026-06-30');
  assert.equal(p.hhmm, '05:00');
}

// --- decideActions ---
const base = { onTime: '05:30', offTime: '22:16', enabled: true, lastOn: '2026-06-28', lastOff: '2026-06-28' };
// enabled=false → 아무것도 안 함
assert.deepEqual(
  decideActions({ ...base, enabled: false }, { today: '2026-06-29', hhmm: '05:30' }),
  { fireOn: false, fireOff: false });
// onTime 도달 & 오늘 미발행 → fireOn
assert.deepEqual(
  decideActions(base, { today: '2026-06-29', hhmm: '05:30' }),
  { fireOn: true, fireOff: false });
// onTime 이전 → 발행 안 함
assert.deepEqual(
  decideActions(base, { today: '2026-06-29', hhmm: '05:29' }),
  { fireOn: false, fireOff: false });
// 오늘 이미 ON 발행됨(lastOn=today) → 재발행 안 함
assert.deepEqual(
  decideActions({ ...base, lastOn: '2026-06-29' }, { today: '2026-06-29', hhmm: '06:00' }),
  { fireOn: false, fireOff: false });
// offTime 도달 & 오늘 미발행 → fireOff (onTime은 이미 발행 가정)
assert.deepEqual(
  decideActions({ ...base, lastOn: '2026-06-29' }, { today: '2026-06-29', hhmm: '22:16' }),
  { fireOn: false, fireOff: true });

// --- validateScheduleInput ---
assert.deepEqual(
  validateScheduleInput({ onTime: '05:30', offTime: '22:16', enabled: true }),
  { ok: true, value: { onTime: '05:30', offTime: '22:16', enabled: true } });
assert.equal(validateScheduleInput({ onTime: '5:30', offTime: '22:16', enabled: true }).ok, false); // zero-pad 안 됨
assert.equal(validateScheduleInput({ onTime: '25:00', offTime: '22:16', enabled: true }).ok, false); // 시 범위 초과
assert.equal(validateScheduleInput({ onTime: '05:30', offTime: '22:16', enabled: 'yes' }).ok, false); // boolean 아님
assert.equal(validateScheduleInput({ onTime: '05:30', enabled: true }).ok, false); // offTime 누락

console.log('schedule tests passed');
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `node worker/src/schedule.test.mjs`
Expected: FAIL — `Cannot find module '.../worker/src/schedule.js'`

- [ ] **Step 3: 최소 구현 작성**

Create `worker/src/schedule.js`:

```javascript
// Sentry 스케줄 순수 로직 — I/O 없음(테스트 가능).
// 시각은 KST(UTC+9) 기준 zero-padded "HH:MM" 문자열로 다룬다.

export const DEFAULT_SCHEDULE = {
  onTime: '05:30',
  offTime: '22:16',
  enabled: true,
  lastOn: '',
  lastOff: '',
};

const pad2 = (n) => String(n).padStart(2, '0');

// epochMs(UTC) → KST 날짜/시각 조각. UTC+9를 더한 뒤 UTC 게터로 읽는다.
export function kstParts(epochMs) {
  const d = new Date(epochMs + 9 * 60 * 60 * 1000);
  const today = `${d.getUTCFullYear()}-${pad2(d.getUTCMonth() + 1)}-${pad2(d.getUTCDate())}`;
  const hhmm = `${pad2(d.getUTCHours())}:${pad2(d.getUTCMinutes())}`;
  return { today, hhmm };
}

// 발행 여부 판정. KV 갱신(부수효과)은 호출측(index.js)이 한다.
export function decideActions(state, nowParts) {
  if (!state.enabled) return { fireOn: false, fireOff: false };
  const { today, hhmm } = nowParts;
  const fireOn = hhmm >= state.onTime && state.lastOn !== today;
  const fireOff = hhmm >= state.offTime && state.lastOff !== today;
  return { fireOn, fireOff };
}

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

// 폰이 보낸 설정 검증. lastOn/lastOff는 받지 않는다(Worker 전용).
export function validateScheduleInput(obj) {
  if (obj == null || typeof obj !== 'object') return { ok: false, error: 'body must be an object' };
  const { onTime, offTime, enabled } = obj;
  if (!HHMM_RE.test(onTime)) return { ok: false, error: 'onTime must be HH:MM' };
  if (!HHMM_RE.test(offTime)) return { ok: false, error: 'offTime must be HH:MM' };
  if (typeof enabled !== 'boolean') return { ok: false, error: 'enabled must be boolean' };
  return { ok: true, value: { onTime, offTime, enabled } };
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `node worker/src/schedule.test.mjs`
Expected: `schedule tests passed` (exit 0)

- [ ] **Step 5: 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
git add worker/src/schedule.js worker/src/schedule.test.mjs
git commit -m "feat: Worker용 Sentry 스케줄 순수 로직(KST 변환·판정·검증)"
```

---

## Task 2: Worker 엔트리 + 배포 설정 (`index.js`, `wrangler.toml`)

KV·ntfy I/O와 Cron 트리거를 묶는 통합 작업이다. 순수 로직은 Task 1에서 검증했고, 여기서는 `wrangler dev`로 수동 검증한다.

**Files:**
- Create: `worker/src/index.js`
- Create: `worker/wrangler.toml`
- Create: `worker/README.md`

**Interfaces:**
- Consumes (Task 1): `DEFAULT_SCHEDULE`, `kstParts`, `decideActions`, `validateScheduleInput`.
- Produces (Task 4가 사용하는 HTTP 계약):
  - `GET /api/sentry-schedule` → `200` JSON `{ onTime, offTime, enabled, lastOn, lastOff }`
  - `PUT /api/sentry-schedule` (헤더 `Authorization: Bearer <token>`, 본문 `{ onTime, offTime, enabled }`)
    → 성공 `200 { ok: true, value }`, 토큰 불일치 `401`, 검증 실패 `400 { error }`
  - `OPTIONS` → `204` CORS preflight

- [ ] **Step 1: Worker 엔트리 작성**

Create `worker/src/index.js`:

```javascript
import {
  DEFAULT_SCHEDULE, kstParts, decideActions, validateScheduleInput,
} from './schedule.js';

const KV_KEY = 'sentry-schedule';

const CORS = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Methods': 'GET, PUT, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type, Authorization',
};

const json = (obj, status = 200) =>
  new Response(JSON.stringify(obj), {
    status,
    headers: { 'Content-Type': 'application/json', ...CORS },
  });

async function readState(env) {
  const raw = await env.SCHEDULE_KV.get(KV_KEY);
  if (!raw) return { ...DEFAULT_SCHEDULE };
  try {
    return { ...DEFAULT_SCHEDULE, ...JSON.parse(raw) };
  } catch {
    return { ...DEFAULT_SCHEDULE };
  }
}

async function writeState(env, state) {
  await env.SCHEDULE_KV.put(KV_KEY, JSON.stringify(state));
}

async function publishNtfy(env, text) {
  await fetch(env.NTFY_URL, { method: 'POST', body: text });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (request.method === 'OPTIONS') return new Response(null, { status: 204, headers: CORS });
    if (url.pathname !== '/api/sentry-schedule') return new Response('not found', { status: 404, headers: CORS });

    if (request.method === 'GET') {
      return json(await readState(env));
    }

    if (request.method === 'PUT') {
      const auth = request.headers.get('Authorization') || '';
      if (auth !== `Bearer ${env.SENTRY_TOKEN}`) return json({ error: 'unauthorized' }, 401);
      let body;
      try { body = await request.json(); } catch { return json({ error: 'invalid json' }, 400); }
      const v = validateScheduleInput(body);
      if (!v.ok) return json({ error: v.error }, 400);
      const state = await readState(env);
      const next = { ...state, ...v.value, updatedAt: new Date().toISOString() };
      await writeState(env, next);
      return json({ ok: true, value: v.value });
    }

    return json({ error: 'method not allowed' }, 405);
  },

  async scheduled(event, env) {
    const state = await readState(env);
    const now = kstParts(Date.now());
    const { fireOn, fireOff } = decideActions(state, now);
    let changed = false;
    if (fireOn) { await publishNtfy(env, 'sentry on'); state.lastOn = now.today; changed = true; }
    if (fireOff) { await publishNtfy(env, 'sentry off'); state.lastOff = now.today; changed = true; }
    if (changed) await writeState(env, state); // 발행할 때만 KV 쓰기(과금 보호)
  },
};
```

- [ ] **Step 2: wrangler 설정 작성**

Create `worker/wrangler.toml` (`<KV_NAMESPACE_ID>`는 Step 4에서 채운다):

```toml
name = "tesla-sentry-scheduler"
main = "src/index.js"
compatibility_date = "2026-06-29"

[triggers]
crons = ["* * * * *"]   # 1분마다 scheduled() 실행 (시각은 KV 데이터에서 읽음)

[[kv_namespaces]]
binding = "SCHEDULE_KV"
id = "<KV_NAMESPACE_ID>"

[vars]
NTFY_URL = "https://ntfy.sh/tesla-ab-9f3k7q2zx8m"
```

- [ ] **Step 3: 배포 절차 문서 작성**

Create `worker/README.md`:

````markdown
# tesla-sentry-scheduler (Cloudflare Worker)

폰이 보낸 Sentry 스케줄(KV)을 매 분 판정해 ntfy로 `sentry on`/`sentry off`를 발행한다.

## 최초 배포

```bash
cd worker

# 1) KV 네임스페이스 생성 → 출력된 id를 wrangler.toml의 <KV_NAMESPACE_ID>에 기입
npx wrangler kv namespace create SCHEDULE_KV

# 2) PUT 검증 시크릿 주입 (긴 랜덤 문자열). 같은 값을 PWA app.js의 SENTRY_TOKEN에도 넣는다.
#    예: openssl rand -hex 16 으로 생성
npx wrangler secret put SENTRY_TOKEN

# 3) 배포 → 출력되는 https://tesla-sentry-scheduler.<sub>.workers.dev 를 기록(PWA에 기입)
npx wrangler deploy
```

## 로컬 검증

```bash
# fetch 핸들러 확인
npx wrangler dev
#   GET:  curl http://localhost:8787/api/sentry-schedule
#   PUT:  curl -X PUT http://localhost:8787/api/sentry-schedule \
#           -H 'Authorization: Bearer <token>' -H 'Content-Type: application/json' \
#           -d '{"onTime":"05:30","offTime":"22:16","enabled":true}'

# Cron(scheduled) 핸들러 강제 실행
npx wrangler dev --test-scheduled
#   curl "http://localhost:8787/__scheduled?cron=*+*+*+*+*"
```

ntfy 토픽 변경 시 `wrangler.toml`의 `NTFY_URL` 수정.
````

- [ ] **Step 4: 배포 + 수동 검증**

```bash
cd worker
npx wrangler kv namespace create SCHEDULE_KV   # id를 wrangler.toml에 기입
npx wrangler secret put SENTRY_TOKEN           # 랜덤 토큰 입력(기록해 둘 것)
npx wrangler deploy                            # workers.dev URL 기록
```

검증 (배포 URL을 `$W`, 토큰을 `$T`로):
- `curl $W/api/sentry-schedule` → 기본값 JSON(`onTime:"05:30"...`) 반환
- 토큰 없는 PUT → `401`:
  `curl -X PUT $W/api/sentry-schedule -H 'Content-Type: application/json' -d '{"onTime":"06:00","offTime":"22:00","enabled":true}'`
- 토큰 있는 PUT → `200 {ok:true}`, 이후 GET이 `06:00/22:00` 반영:
  `curl -X PUT $W/api/sentry-schedule -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"onTime":"06:00","offTime":"22:00","enabled":true}'`
- 잘못된 시각 PUT → `400`:
  `curl -X PUT $W/api/sentry-schedule -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -d '{"onTime":"25:00","offTime":"22:00","enabled":true}'`

Expected: 위 각 상태코드/응답이 그대로 나온다.

- [ ] **Step 5: 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
git add worker/src/index.js worker/wrangler.toml worker/README.md
git commit -m "feat: Sentry 스케줄 Worker(GET/PUT API + 1분 Cron 발행)"
```

> 참고: `worker/wrangler.toml`에 KV `id`가 들어간다. 비밀이 아니라 커밋해도 무방하다. 시크릿(`SENTRY_TOKEN`)은 `wrangler secret`으로만 관리하며 저장소에 넣지 않는다.

---

## Task 3: PWA 순수 로직 (`sentry.js`)

**Files:**
- Create: `webapp/sentry.js`
- Test: `webapp/sentry.test.mjs`

**Interfaces:**
- Consumes: 없음.
- Produces (Task 4가 사용):
  - `isValidHHMM(s: string)` → `boolean`
  - `buildSchedulePayload({ onTime, offTime, enabled })` → `{ onTime, offTime, enabled: boolean }`

- [ ] **Step 1: 실패하는 테스트 작성**

Create `webapp/sentry.test.mjs`:

```javascript
import assert from 'node:assert/strict';
import { isValidHHMM, buildSchedulePayload } from './sentry.js';

assert.equal(isValidHHMM('05:30'), true);
assert.equal(isValidHHMM('23:59'), true);
assert.equal(isValidHHMM('5:30'), false);   // zero-pad 안 됨
assert.equal(isValidHHMM('24:00'), false);  // 시 범위 초과
assert.equal(isValidHHMM('05:60'), false);  // 분 범위 초과
assert.equal(isValidHHMM(''), false);

assert.deepEqual(
  buildSchedulePayload({ onTime: '05:30', offTime: '22:16', enabled: true }),
  { onTime: '05:30', offTime: '22:16', enabled: true });
// enabled를 명시적 boolean으로 강제
assert.deepEqual(
  buildSchedulePayload({ onTime: '06:00', offTime: '21:00', enabled: 1 }),
  { onTime: '06:00', offTime: '21:00', enabled: true });
assert.deepEqual(
  buildSchedulePayload({ onTime: '06:00', offTime: '21:00', enabled: 0 }),
  { onTime: '06:00', offTime: '21:00', enabled: false });

console.log('sentry tests passed');
```

- [ ] **Step 2: 테스트 실패 확인**

Run: `node webapp/sentry.test.mjs`
Expected: FAIL — `Cannot find module '.../webapp/sentry.js'`

- [ ] **Step 3: 최소 구현 작성**

Create `webapp/sentry.js`:

```javascript
// PWA Sentry 섹션과 단위 테스트가 공유하는 순수 함수. DOM 접근 없음.

const HHMM_RE = /^([01]\d|2[0-3]):[0-5]\d$/;

export function isValidHHMM(s) {
  return HHMM_RE.test(s);
}

// Worker PUT 본문을 만든다. enabled는 명시적 boolean으로 정규화.
export function buildSchedulePayload({ onTime, offTime, enabled }) {
  return { onTime, offTime, enabled: !!enabled };
}
```

- [ ] **Step 4: 테스트 통과 확인**

Run: `node webapp/sentry.test.mjs`
Expected: `sentry tests passed` (exit 0)

- [ ] **Step 5: 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
git add webapp/sentry.js webapp/sentry.test.mjs
git commit -m "feat: PWA Sentry 시각 검증·페이로드 빌드 순수 함수"
```

---

## Task 4: PWA UI 통합 (`index.html`, `style.css`, `app.js`)

기존 애프터블로우 화면 하단에 Sentry 섹션을 붙이고, 로드 시 GET·저장 시 PUT을 연결한다. 브라우저로 수동 검증한다.

**Files:**
- Modify: `webapp/index.html` (애프터블로우 카드 내부, `note` 단락 뒤)
- Modify: `webapp/style.css` (Sentry 섹션 스타일 추가)
- Modify: `webapp/app.js` (Sentry 로드/저장 로직 추가)

**Interfaces:**
- Consumes (Task 3): `isValidHHMM`, `buildSchedulePayload`.
- Consumes (Task 2): `GET`/`PUT /api/sentry-schedule` 계약.

- [ ] **Step 1: HTML에 Sentry 섹션 추가**

In `webapp/index.html`, `<p class="note">...</p>`(33번째 줄) 바로 뒤, `</main>` 앞에 삽입:

```html
    <hr class="sep">

    <h2 class="sub">감시 모드 (Sentry)</h2>

    <section class="row toggle">
      <span class="label">자동 스케줄</span>
      <label class="switch">
        <input id="sentryEnabled" type="checkbox">
        <span class="track"></span>
      </label>
    </section>

    <section class="row">
      <span class="label">켜는 시각</span>
      <input id="sentryOn" type="time" class="time">
    </section>
    <section class="row">
      <span class="label">끄는 시각</span>
      <input id="sentryOff" type="time" class="time">
    </section>

    <button id="sentrySave" class="start">설정 저장</button>
    <p id="sentryStatus" class="status"></p>
```

- [ ] **Step 2: CSS 추가**

In `webapp/style.css`, 파일 끝에 추가:

```css
.sep { border: none; border-top: 1px solid #334155; margin: 28px 0 20px; }
.sub { margin: 0 0 16px; font-size: 1.15rem; text-align: center; }
.time {
  font-size: 1.2rem; font-weight: 700; color: var(--fg);
  background: #0f172a; border: 1px solid #475569; border-radius: 10px;
  padding: 8px 12px;
}
#sentrySave { margin-top: 20px; }
```

- [ ] **Step 3: app.js에 로드/저장 로직 추가**

In `webapp/app.js`, 상단 import와 상수에 추가하고(파일 맨 위 `import` 아래), 서비스워커 등록 직전에 Sentry 로직을 넣는다.

import 줄을 다음으로 교체:

```javascript
import { buildMessage } from './message.js';
import { isValidHHMM, buildSchedulePayload } from './sentry.js';
```

`const TOPIC_URL = ...` 아래에 추가 (배포 후 실제 값으로 교체):

```javascript
// Task 2에서 배포한 Worker URL과 시크릿 토큰. 배포 산출 값으로 교체한다.
const SENTRY_API = 'https://tesla-sentry-scheduler.<sub>.workers.dev/api/sentry-schedule';
const SENTRY_TOKEN = '<wrangler secret put SENTRY_TOKEN 에 넣은 값과 동일>';
const SENTRY_CACHE_KEY = 'sentry-schedule';
```

서비스워커 등록 블록(`if ('serviceWorker' in navigator) {...}`) 바로 위에 추가:

```javascript
const sentryEnabled = document.getElementById('sentryEnabled');
const sentryOn = document.getElementById('sentryOn');
const sentryOff = document.getElementById('sentryOff');
const sentrySave = document.getElementById('sentrySave');
const sentryStatusEl = document.getElementById('sentryStatus');

function setSentryStatus(text, kind) {
  sentryStatusEl.textContent = text;
  sentryStatusEl.className = `status ${kind}`;
}

function applySchedule(s) {
  sentryOn.value = s.onTime;
  sentryOff.value = s.offTime;
  sentryEnabled.checked = !!s.enabled;
}

async function loadSchedule() {
  try {
    const res = await fetch(SENTRY_API);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const s = await res.json();
    applySchedule(s);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(s));
  } catch (e) {
    const cached = localStorage.getItem(SENTRY_CACHE_KEY);
    if (cached) applySchedule(JSON.parse(cached));
    setSentryStatus(`현재 설정을 못 불러왔습니다 (${e.message}) — 캐시 표시`, 'err');
  }
}

async function saveSchedule() {
  if (!isValidHHMM(sentryOn.value) || !isValidHHMM(sentryOff.value)) {
    setSentryStatus('시각 형식이 올바르지 않습니다', 'err');
    return;
  }
  const payload = buildSchedulePayload({
    onTime: sentryOn.value, offTime: sentryOff.value, enabled: sentryEnabled.checked,
  });
  sentrySave.disabled = true;
  setSentryStatus('저장 중…', 'pending');
  try {
    const res = await fetch(SENTRY_API, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${SENTRY_TOKEN}` },
      body: JSON.stringify(payload),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    localStorage.setItem(SENTRY_CACHE_KEY, JSON.stringify(payload));
    setSentryStatus('저장됨 ✓', 'ok');
  } catch (e) {
    setSentryStatus(`저장 실패: ${e.message} — 다시 시도하세요`, 'err');
  } finally {
    sentrySave.disabled = false;
  }
}

sentrySave.addEventListener('click', saveSchedule);
loadSchedule();
```

- [ ] **Step 4: 순수 로직 회귀 + 브라우저 수동 검증**

먼저 기존 JS 테스트가 여전히 통과하는지:

Run: `node webapp/message.test.mjs && node webapp/sentry.test.mjs`
Expected: `message tests passed` / `sentry tests passed`

브라우저 검증 (Task 2 Worker 배포 완료 + app.js 상수 기입 후):
- 로컬 정적 서버로 webapp 열기: `python3 -m http.server -d webapp 8080` → `http://localhost:8080`
- 화면 하단에 Sentry 섹션 표시, 로드 시 켜는/끄는 시각이 Worker의 현재값으로 채워짐
- 시각 변경 후 "설정 저장" → "저장됨 ✓", `curl $W/api/sentry-schedule`에 반영 확인
- 다른 탭/기기에서 새로 열면 같은 값으로 채워짐(동기화)

- [ ] **Step 5: PWA 배포 + 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
npx wrangler pages deploy webapp
git add webapp/index.html webapp/style.css webapp/app.js
git commit -m "feat: 애프터블로우 화면에 Sentry 스케줄 설정 UI 추가"
```

---

## Task 5: PC 리스너 sentry 분기 (`afterblow-lib.sh`, `afterblow-listener.sh`)

**Files:**
- Modify: `scripts/afterblow-lib.sh` (`ab_parse_sentry` 추가)
- Create: `scripts/test-sentry-parse.sh`
- Modify: `scripts/afterblow-listener.sh` (`ab_consume_stream`에 sentry 분기)

**Interfaces:**
- Consumes: 기존 `ab_dispatch_line`(afterblow 경로), `tesla-sentry on|off` CLI.
- Produces:
  - `ab_parse_sentry <line>` → 첫 토큰이 `sentry`이고 둘째가 `on|off`이면 `on`/`off`를 출력하고 return 0. 아니면 return 1.

- [ ] **Step 1: 실패하는 파서 테스트 작성**

Create `scripts/test-sentry-parse.sh`:

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

check "sentry on"  "on"  "$(ab_parse_sentry 'sentry on')"
check "sentry off" "off" "$(ab_parse_sentry 'sentry off')"

# 비대상/불량 입력은 return 1 (출력 없음)
for bad in 'sentry maybe' 'sentry' 'afterblow 2' 'hello' 'sentry on extra'; do
	if ab_parse_sentry "$bad" >/dev/null 2>&1; then
		printf 'FAIL - rejected: %s\n' "$bad"; fail=1
	else
		printf 'ok   - rejected: %s\n' "$bad"
	fi
done

exit "$fail"
```

참고: `sentry on extra`처럼 토큰이 3개 이상이면 거부한다(엄격하게 on/off 단독만 허용).

- [ ] **Step 2: 테스트 실패 확인**

Run: `bash scripts/test-sentry-parse.sh`
Expected: FAIL — `ab_parse_sentry: command not found` 류로 실패(exit 1)

- [ ] **Step 3: `ab_parse_sentry` 구현**

In `scripts/afterblow-lib.sh`, 파일 끝(`ab_dispatch_line` 뒤)에 추가:

```bash
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
```

- [ ] **Step 4: 파서 테스트 통과 확인**

Run: `bash scripts/test-sentry-parse.sh`
Expected: 모든 줄 `ok` (exit 0)

- [ ] **Step 5: 리스너 스트림에 sentry 분기 추가**

In `scripts/afterblow-listener.sh`, sentry 실행 래퍼를 추가하고 `ab_consume_stream`을 수정한다.

`run_handler()` 정의(18번째 줄) 바로 아래에 추가:

```bash
# sentry on|off 실행 래퍼.
run_sentry() { "$ROOT/tesla-sentry" "$1" >>"$LOG" 2>&1 || log "tesla-sentry $1 exited non-zero"; }
```

`ab_consume_stream` 안의 디스패치 줄을 수정한다. 기존:

```bash
		log "recv: $line"
		ab_dispatch_line "$line" "$handler" || true
```

다음으로 교체:

```bash
		log "recv: $line"
		local sentry_arg
		if sentry_arg="$(ab_parse_sentry "$line")"; then
			log "sentry $sentry_arg"
			run_sentry "$sentry_arg"
		else
			ab_dispatch_line "$line" "$handler" || true
		fi
```

- [ ] **Step 6: 리스너 분기 수동 검증**

`ab_consume_stream`이 sentry/afterblow를 올바로 가르는지, 가짜 핸들러·가짜 `tesla-sentry`로 확인:

```bash
cd "$(git rev-parse --show-toplevel)"
tmp="$(mktemp -d)"
# 가짜 tesla-sentry: 인자를 기록
printf '#!/usr/bin/env bash\necho "SENTRY $*" >> "%s/out"\n' "$tmp" > "$tmp/tesla-sentry"
chmod +x "$tmp/tesla-sentry"
# 리스너를 source하되 ROOT/HANDLER를 가짜로 덮어쓰고 main 루프는 돌리지 않음
LOG=/dev/null bash -c '
  DIR="scripts"; source "$DIR/afterblow-lib.sh"
  ROOT="'"$tmp"'"; LOG=/dev/null
  log(){ :; }
  run_sentry(){ "$ROOT/tesla-sentry" "$1"; }
  fake_afterblow(){ echo "AFTERBLOW $*" >> "'"$tmp"'/out"; }
  # listener의 consume 로직만 인라인 재현 대신 함수 사용:
  source "$DIR/afterblow-listener.sh" 2>/dev/null || true
  printf "sentry on\nafterblow 2 vent\nsentry off\nhello\n" | ab_consume_stream fake_afterblow
'
cat "$tmp/out"
```

Expected `out` 내용(순서대로):
```
SENTRY on
AFTERBLOW 2 vent
SENTRY off
```
(`hello`는 무시되어 출력 없음)

> 검증이 환경에 따라 까다로우면, 최소한 다음 두 가지만 확인해도 된다:
> `ab_parse_sentry 'sentry on'` → `on`, `ab_parse_sentry 'afterblow 2'` → exit 1.

- [ ] **Step 7: 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
git add scripts/afterblow-lib.sh scripts/test-sentry-parse.sh scripts/afterblow-listener.sh
git commit -m "feat: PC 리스너가 sentry on/off 신호를 분기 실행"
```

---

## Task 6: crontab 정리 + 운영 메모

Worker Cron이 sentry 스케줄을 대체하므로 PC crontab의 sentry 줄을 제거한다. 이 작업은 사용자 환경(crontab)에 대한 것이라 자동 편집하지 않고 안내·확인한다.

**Files:**
- Modify: `README.md` (sentry crontab 관련 설명을 Worker 방식으로 갱신 — 해당 섹션이 있으면)

- [ ] **Step 1: 현재 crontab 확인**

Run: `crontab -l`
Expected: 다음 두 줄(또는 유사)이 보인다 —
```
30 5  * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
16 22 * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
```

- [ ] **Step 2: sentry on/off 두 줄 제거**

`crontab -e`로 위 **sentry on/off 두 줄만** 삭제한다(애프터블로우/기타 줄은 유지). 편집 후:

Run: `crontab -l | grep -c 'tesla-sentry o[nf]'`
Expected: `0`

- [ ] **Step 3: README 갱신**

`README.md`에서 sentry를 crontab으로 켜고/끈다고 설명한 부분이 있으면, "Cloudflare Worker Cron이 KV의 설정 시각을 읽어 ntfy로 발행하고 PC 리스너가 실행한다. 시각은 PWA 화면에서 변경한다"로 갱신한다. 애프터블로우 listener 설명에 `sentry on/off` 분기 처리도 한 줄 덧붙인다.

(README에 해당 내용이 없으면 이 step은 건너뛴다.)

- [ ] **Step 4: 엔드투엔드 확인**

- PWA에서 켜는 시각을 **현재 시각 +2분**으로 설정·저장
- 약 2분 뒤 `tail -f afterblow.log`에 `recv: sentry on` → `sentry on` → tesla-sentry 실행 로그 확인
- 차량에서 Sentry가 켜지는지 확인(또는 `tesla-sentry status`)
- 마스터 토글 OFF로 저장 → 다음 분에 발행이 멈추는지(로그에 sentry 줄 없음) 확인
- 확인 후 켜는/끄는 시각을 원래 운영값(05:30 / 22:16)으로 되돌리고 토글 ON

- [ ] **Step 5: 커밋** (커밋 정책에 따라 사용자 승인 시)

```bash
git add README.md
git commit -m "docs: Sentry 스케줄을 Worker+PWA 방식으로 문서 갱신"
```

---

## Self-Review (작성자 점검 결과)

- **스펙 커버리지**: KV 데이터 모델→Task 1·2 / Worker GET·PUT·Cron→Task 2 / KST·오늘-가드→Task 1 / 입력 검증→Task 1·2·3·5 / PWA UI·동기화→Task 3·4 / PC 분기→Task 5 / crontab 제거→Task 6 / ntfy 토픽 공유→Task 2(`NTFY_URL`)·5(파서). 누락 없음.
- **타입 일관성**: `decideActions`는 `{fireOn, fireOff}`로 Task 1 정의·Task 2 사용 일치. `validateScheduleInput`→`{ok,value|error}` 일치. `buildSchedulePayload`/`isValidHHMM` Task 3 정의·Task 4 사용 일치. `ab_parse_sentry`는 `on`/`off` 출력·return 0/1로 Task 5 내 정의·사용 일치.
- **플레이스홀더**: Worker URL/토큰은 배포 산출 환경값이라 불가피하며, Task 2에서 생성·Task 4에서 기입하도록 순서와 출처를 명시(임의 TODO 아님).
- **시각 비교**: 모든 경로에서 zero-padded `HH:MM`만 통과시키므로 사전순 비교 안전(Global Constraints에 명시).
