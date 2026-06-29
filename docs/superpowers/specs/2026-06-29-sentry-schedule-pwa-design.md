# 감시 모드(Sentry) 스케줄 PWA 제어 — 설계 문서

작성일: 2026-06-29

## 배경 / 목적

현재 Sentry(테슬라 감시 모드)는 `tesla-sentry on/off` CLI로만 켜고 끄며,
**PC의 crontab에 시각이 하드코딩**되어 있다(매일 05:30 ON, 22:16 OFF).
시각을 바꾸려면 PC에서 crontab을 직접 편집해야 하고, **폰에서는 손댈 수 없다.**

이를 애프터블로우와 동일하게 **폰(PWA)에서 ON/OFF 시각을 설정**하고,
그 설정이 자동 스케줄에 **즉시 반영**되도록 한다. 또한 **여러 기기에서 같은
설정을 보고 바꿀 수 있어야** 하므로(부부 2인, 폰 여러 대), 설정을 클라우드에
단일 진실(single source of truth)로 둔다.

비배포 / 비상업 / 개인용. Play 스토어 등록 없음.

## 핵심 설계 결정 (브레인스토밍 합의)

1. **스케줄 범위**: 매일 동일한 ON/OFF 시각 2개 + 마스터 활성 토글. 요일별 차등 없음.
2. **비활성화 토글 = 마스터 스위치**: `enabled=false`이면 **자동 스케줄 자체가 정지**
   (ON/OFF 둘 다 발행 안 함). 이는 "오늘만 스킵"도 "강제 OFF"도 아니다.
   다시 켜기 전까지 무기한 정지이며, 끌 당시의 차량 상태는 그대로 둔다.
3. **단일 진실 = Cloudflare KV**: 다기기 동기화를 위해 설정을 KV에 저장. PC는 공인 IP가
   없어(outbound만) 폰이 PC를 직접 읽을 수 없으므로, 읽기 가능한 클라우드 저장소가 필요.
4. **시각 판정 = Cloudflare Worker Cron**: Worker가 1분마다 KV를 읽어 시각 도달을 판정하고
   ntfy로 신호 발행. **PC의 crontab이 아니다**(아래 "Worker Cron 정의" 참조).
5. **실행 = PC**: 테슬라 명령 서명용 `private-key.pem`이 PC에만 있으므로, 실제 on/off는
   항상 PC에서 일어난다. PC는 ntfy 신호를 받아 `tesla-sentry on/off`를 호출하는 수신·실행만 담당.
6. **시간대 = KST 고정**(Asia/Seoul). Worker는 UTC로 동작하므로 내부에서 +9h 변환해 비교.
7. **PC 데몬 = 기존 bash listener 확장**: 타이머가 Worker로 빠져 PC는 단순 수신기이므로,
   동작 중인 `afterblow-listener.sh`를 재작성하지 않고 메시지 분기만 추가한다.

### "Worker Cron"의 정의 (혼동 주의)

PC의 crontab과 **무관**하다. Cloudflare Worker의 **Cron Triggers** 기능으로,
Cloudflare 클라우드가 주기적으로 Worker 코드를 깨워 실행한다.

```toml
# wrangler.toml
[triggers]
crons = ["* * * * *"]   # "1분마다 Worker를 실행하라" — 이 표현식은 고정
```

cron 표현식(`* * * * *`)에는 **사용자의 ON/OFF 시각이 들어가지 않는다.** 시각은
KV 데이터에 있고, Worker가 매 분 그 데이터를 읽어 판정한다. 따라서 사용자가 시각을
바꿀 때 **cron 표현식이나 crontab을 고치지 않고, KV 값만 바꾼다.**

## 범위

작업은 세 부분이다.

1. **Cloudflare Worker + KV** — 설정 저장소(KV)와 설정 API(GET/PUT) + 스케줄 판정(Cron).
2. **폰 PWA** — 기존 애프터블로우 화면에 "감시 모드(Sentry)" 섹션 추가. KV를 읽어 현재
   설정을 표시하고, 변경 시 KV에 저장.
3. **PC 수신측 수정** — bash listener가 `sentry on/off` 신호도 분기 처리. crontab의
   기존 sentry 줄 제거.

비목표(YAGNI):
- 요일별/공휴일별 차등 스케줄.
- 즉시 on/off 버튼, 차량 상태 실시간 표시(스케줄 설정만 다룬다).
- 사용자 계정/인증 체계(공유 시크릿 토큰으로 충분, 아래 "보안" 참조).
- 놓친 신호의 정교한 재시도 큐(오늘-가드로 그날 1회 보장하는 수준까지만).

## 전체 구조

```
[폰 ×N]  PWA (Cloudflare Pages, 애프터블로우 화면에 Sentry 섹션 추가)
  │  화면 열 때: GET /api/sentry-schedule        → 현재 설정 표시(다기기 동기화)
  │  저장 시:    PUT /api/sentry-schedule (토큰)  → onTime/offTime/enabled 갱신
  ▼
[Cloudflare Worker + KV]   ── 단일 진실: { onTime, offTime, enabled, lastOn, lastOff }
  │  Cron 1분마다 scheduled():
  │    KV 읽기 → enabled면 KST 현재시각으로 판정
  │    now>=onTime  & lastOn !=오늘  → ntfy "sentry on",  lastOn =오늘 저장
  │    now>=offTime & lastOff!=오늘  → ntfy "sentry off", lastOff=오늘 저장
  ▼
[ntfy.sh]  tesla-ab-9f3k7q2zx8m   (애프터블로우와 동일 토픽)
  │
  ▼
[PC]  afterblow-listener.sh (메시지 분기)
        ├─ "afterblow ..."  → 기존 처리(afterblow-run.sh)
        └─ "sentry on|off"  → tesla-sentry on | off
```

폰은 Worker로 outbound HTTP만, PC는 ntfy outbound 구독만 사용하므로 공인 IP·포트개방이
필요 없다(기존과 동일).

## 데이터 모델 (Cloudflare KV)

- KV 네임스페이스 1개, 키 1개: `sentry-schedule`
- 값(JSON):

```json
{
  "onTime":  "05:30",
  "offTime": "22:16",
  "enabled": true,
  "lastOn":  "2026-06-29",
  "lastOff": "2026-06-29",
  "updatedAt": "2026-06-29T01:30:00Z"
}
```

- `onTime` / `offTime`: `HH:MM` 24시간제, **KST 기준**. 폰이 PUT으로 설정.
- `enabled`: 마스터 스위치. 폰이 PUT으로 설정.
- `lastOn` / `lastOff`: `YYYY-MM-DD`(KST). **Worker만 갱신**(오늘 발행 가드용). 폰은 안 건드림.
- `updatedAt`: 마지막 PUT 시각(디버깅용).
- **최초 비어있을 때 기본값**: `onTime=05:30`, `offTime=22:16`, `enabled=true`
  (기존 crontab 값과 동일). Worker가 GET/Cron 모두에서 이 기본값을 적용.

## Cloudflare Worker

단일 Worker가 HTTP API와 Cron 두 역할을 담당.

### HTTP API (`fetch` 핸들러)

- `GET /api/sentry-schedule`
  - KV 값을 그대로 반환(없으면 기본값). 폰이 현재 설정을 표시하는 데 사용 → 다기기 동기화의 핵심.
  - 인증 불필요(읽기 전용, 설정값은 민감하지 않음). 단순화를 위해 GET은 공개.
- `PUT /api/sentry-schedule`
  - `Authorization` 또는 커스텀 헤더로 **공유 시크릿 토큰** 검증. 불일치 시 401.
  - 본문에서 `onTime`, `offTime`, `enabled`만 받아 검증 후 KV에 병합 저장
    (`lastOn/lastOff`는 보존). `updatedAt` 갱신.
  - **입력 검증**(공개 URL이므로 신뢰 불가): `HH:MM` 형식·범위(00:00–23:59) 확인,
    `enabled`는 boolean. 잘못된 입력은 400.
- CORS: PWA(Pages 도메인)에서 호출하므로 적절한 CORS 헤더 허용.

### Cron 판정 (`scheduled` 핸들러, 매 분)

```
state = KV.get("sentry-schedule")  // 없으면 기본값
if (!state.enabled) return                       // 마스터 OFF면 아무것도 안 함

nowKST = new Date(Date.now() + 9h)
today  = "YYYY-MM-DD" (KST)
hhmm   = "HH:MM" (KST)

if (hhmm >= state.onTime  && state.lastOn  !== today) {
    publishNtfy("sentry on");  state.lastOn  = today;  KV.put(state)
}
if (hhmm >= state.offTime && state.lastOff !== today) {
    publishNtfy("sentry off"); state.lastOff = today;  KV.put(state)
}
```

- **오늘-가드(`lastOn/lastOff`)의 의도**: 정확히 "그 1분"만 매칭하지 않고 `>= 시각 &&
  오늘 미발행` 방식이라, Cron이 몇 분 지연·누락돼도 **그날 1회는 보장**한다. 동시에
  같은 날 중복 발행을 막는다.
- **시각 변경과의 상호작용**: 오늘 이미 05:30에 켜진 뒤(`lastOn=오늘`) ON을 06:00으로
  바꿔도 다시 켜지지 않는다. 반대로 아직 안 켜진 상태에서 시각을 당기면 그 분에 켜진다.
- **자정 넘김 미지원(가정)**: `onTime < offTime`(같은 날 안에서 켜고 끔)을 전제로 한다.
  현재 사용 패턴(05:30/22:16)에 부합. off가 on보다 빠른 설정은 의미상 비정상으로 보고 다루지 않는다.

### 시크릿 / 설정 주입

- `wrangler secret`으로 주입: **PUT 검증 토큰**, **ntfy 토픽 URL**.
- KV 네임스페이스 바인딩은 `wrangler.toml`에 선언.

## 폰 PWA (애프터블로우 화면 확장)

기존 단일 페이지 하단에 "감시 모드(Sentry)" 섹션을 덧붙인다(별도 페이지/탭 없음).

### 화면

```
   ── 감시 모드 (Sentry) ───────────

   자동 스케줄        [ ON ●]      ← enabled 마스터 토글

   켜는 시각   [ 05:30 ]           ← <input type="time">
   끄는 시각   [ 22:16 ]

   [        저장        ]
   ─ 저장됨 ✓   (실패 시 빨강 + 재시도)
```

- 화면 로드 시 `GET /api/sentry-schedule` → 받은 값으로 입력 필드 채움 → 어느 기기서
  열어도 같은 값(동기화). 응답을 localStorage에도 캐시.
- `<input type="time">`로 ON/OFF 시각 입력, 토글로 `enabled`.
- "저장" → `PUT`(시크릿 토큰 헤더, 본문 `{onTime, offTime, enabled}`):
  - 200 → "저장됨 ✓"(초록)
  - 실패/오류 → 빨강 + 재시도 버튼
- **정직한 문구**: "저장됨"은 설정 저장 성공이며, 차량 동작 결과가 아님을 분명히 한다.

### 구성 파일 (기존 webapp/ 확장)

| 파일 | 변경 |
|---|---|
| `index.html` | 하단에 Sentry 섹션 마크업 추가 |
| `app.js` | 로드 시 GET, 저장 시 PUT 로직 추가 |
| `sentry.js`(신규, 선택) | 시각 검증·요청 빌드 등 순수 로직 분리(테스트 용이) |
| `style.css` | Sentry 섹션 스타일(기존 톤 유지) |

- Worker API 베이스 URL과 시크릿 토큰은 `app.js`(또는 `sentry.js`) 상단 상수 한 곳에 둔다.

### 오류 처리

- `GET` 실패(오프라인 등) → localStorage 캐시값으로 채우고 오류 토스트 표시.
- `PUT` 실패 → 빨강 메시지 + 재시도. 저장은 네트워크 필요함을 안내.
- 서비스워커는 정적 자원만 캐시하며 API 요청을 큐잉하지 않는다(과설계 방지).

## PC 쪽 수정

### `scripts/afterblow-listener.sh` (또는 `afterblow-lib.sh`)

- 현재: 첫 토큰이 `afterblow`인 줄만 처리.
- 변경: 메시지 첫 토큰으로 분기.
  - `afterblow ...` → 기존 핸들러(변경 없음).
  - `sentry on`  → `tesla-sentry on`
  - `sentry off` → `tesla-sentry off`
  - 그 외/keepalive → 무시.
- `sentry` 분기는 **on/off 토큰만 허용**(그 외 인자는 무시)하여 신뢰 불가 입력을 방어.
- 중복 신호는 무해(`on`을 두 번 호출해도 idempotent). 애프터블로우와 달리 별도 디바운스 불필요.

### crontab

- 기존 sentry on/off **두 줄을 제거**한다(이제 Worker Cron이 대체). 애프터블로우 관련
  설정/서비스는 그대로 둔다.

## 배포 · 보안

### 배포

- **Worker**: `wrangler deploy`로 배포, `wrangler kv namespace create`로 KV 생성,
  `wrangler secret put`으로 토큰·ntfy URL 주입, `wrangler.toml`에 cron 트리거 선언.
- **PWA**: 기존 `wrangler pages deploy`로 동일 사이트 재배포(Sentry 섹션 포함).
- 이미 프로젝트에 Cloudflare/Wrangler가 세팅되어 있어 그대로 활용.

### 보안

- **GET**: 공개(설정값은 민감하지 않음).
- **PUT**: 공유 시크릿 토큰 헤더로 보호. 토큰은 PWA JS에 노출되지만, 기존 ntfy 토픽이
  이미 "공개 URL 기반 비밀"인 것과 **동일 보안 수준**이며 개인용으로 충분하다.
- 입력 검증(시각 형식·범위, boolean)은 Worker에서 항상 수행한다.
- (옵션, 기본 미적용) 더 잠그려면 Cloudflare Access로 PUT 경로/Pages를 특정 계정에만 허용 가능.

## 테스트

- **Worker 단위(핵심)**:
  - KST 변환: 주어진 UTC에서 올바른 `HH:MM`/`YYYY-MM-DD`(KST) 산출.
  - 판정: `enabled=false`면 무발행. `now>=onTime && lastOn!=오늘`이면 1회 발행 후
    `lastOn` 갱신. 같은 날 재실행 시 재발행 안 함(가드). off도 동일.
  - 입력 검증: 잘못된 `HH:MM`/타입은 400, 정상값은 KV 병합.
- **PWA 단위**: 시각 입력 검증·요청 본문 빌드(순수 로직). GET 응답으로 필드 채움.
- **PC 분기 단위**: `sentry on`→`tesla-sentry on`, `sentry off`→`tesla-sentry off`,
  `afterblow ...`→기존 경로, 비대상 줄→무시.
- **엔드투엔드**: 폰에서 시각 변경·저장 → 다른 기기서 GET 시 반영 확인 → 설정 시각에
  ntfy 신호 → `afterblow.log`/sentry 로그에 수신·실행 확인. 차량 동작은 차를 보며 짧게 확인.

## 합의된 결정값

- 스케줄 범위: 매일 ON/OFF 시각 2개 + 마스터 활성 토글(요일 차등 없음)
- 비활성화 토글: 마스터 스위치(무기한 정지, ON/OFF 둘 다 안 함; 강제 OFF 아님)
- 단일 진실: Cloudflare KV (다기기 동기화)
- 시각 판정: Cloudflare Worker Cron(1분 주기) → ntfy → PC 실행
- 시간대: KST 고정
- ntfy 토픽: 애프터블로우와 동일 토픽 공유, 메시지 prefix로 분기
- PC 데몬: 기존 bash listener 확장(Go 재작성 안 함)
- Worker PUT 보안: 공유 시크릿 토큰
- crontab의 sentry on/off 줄: 제거
