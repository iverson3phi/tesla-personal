# tesla-sentry

Tesla Fleet API를 사용해 정해진 일정에 따라 테슬라 감시모드(Sentry Mode)를 자동으로 켜고 끕니다. 리눅스 PC에서 바이너리를 직접 실행하거나, 휴대폰 PWA 앱에서 ON/OFF 시각을 설정하면 PC crontab(매분 실행)이 Cloudflare KV에서 시각을 읽어 KST 기준으로 판정해 자동으로 동작시킵니다.

추가로, 주차 후 휴대폰 PWA 앱 버튼 한 번으로 에어컨 증발기를 말려 곰팡이·냄새를 예방하는 **애프터블로우(after-blow)** 자동화도 지원합니다 (아래 [애프터블로우 자동화](#애프터블로우-자동화) 참고).

## 사전 준비물

- **Go 1.23 이상** — `go version`이 `go1.23` 이상을 출력해야 합니다
- **테슬라 계정** — 계정에 차량이 등록되어 있어야 합니다
- **Cloudflare 계정** (무료) — 공개키 파일 하나를 HTTPS로 호스팅하는 데 사용합니다 (아래 3단계에서 설명). 애프터블로우 PWA 앱도 같은 Cloudflare Pages에 올립니다.
- **Node.js / npm** — Wrangler CLI(공개키·PWA 배포)에 필요합니다. Ubuntu 기준 `sudo apt-get install -y nodejs npm`
- **bash / curl** — 애프터블로우 구독 데몬에 필요합니다(대부분의 리눅스에 기본 설치). systemd로 상시 실행합니다.

---

## 프로젝트 구조

```
cmd/tesla-sentry/      Go CLI 진입점 (on/off/status/afterblow 등)
internal/              Go 내부 패키지 (tesla SDK 래퍼, oauth, config, keys)
worker/                Cloudflare Worker — 감시모드 스케줄러
  src/schedule.js          KST 시각 판정 로직 (순수 함수)
  src/index.js             GET/PUT API + Cron 핸들러 (현재 cron 비활성)
  wrangler.toml            Worker 설정 (KV 바인딩, Cron 트리거 — 현재 비활성: crons = [])
  README.md                Worker 배포 절차
scripts/               PC 데몬 (bash) + systemd 유닛 + 단위 테스트
  afterblow-lib.sh         ntfy 메시지 파싱 공통 함수 (listener·run이 source)
  afterblow-listener.sh    ntfy 토픽 상시 구독 → afterblow 디스패치 (sentry on/off 분기는 현재 미사용 — Cloudflare cron 복구 시 대비)
  afterblow-run.sh         메시지의 분/환기로 tesla-sentry afterblow 실행
  tesla-afterblow.service  systemd 유닛 (사용자/경로는 본인 환경에 맞게 수정)
  sentry-schedule-lib.sh   판정 순수 함수 (to_min, sentry_should_fire)
  sentry-schedule-check.sh crontab이 매분 실행: KV GET → 판정 → tesla-sentry on/off
  test-afterblow-*.sh      bash 파서 단위 테스트
  test-sentry-parse.sh     sentry 메시지 파서 단위 테스트
  test-sentry-schedule.sh  판정 단위 테스트
webapp/                휴대폰 PWA (Cloudflare Pages 배포용 정적 파일)
  index.html, style.css, app.js   단일 화면 UI — 애프터블로우 + 감시모드 스케줄 섹션
  message.js, message.test.mjs    애프터블로우 메시지 빌더 + 단위 테스트
  sentry.js, sentry.test.mjs      감시모드 스케줄 API 헬퍼 + 단위 테스트
  manifest.json, sw.js, icon.svg  설치형/오프라인 PWA 자원
```

두 자동화는 독립적입니다: **감시모드**(PWA → KV → PC crontab → tesla-sentry 직접 실행)와 **애프터블로우**(PWA → ntfy → bash 데몬). 같은 `tesla-sentry` 바이너리와 `~/.config/tesla-sentry/` 설정·토큰을 공유합니다.

---

## 빌드

```bash
go build -o tesla-sentry ./cmd/tesla-sentry
sudo install tesla-sentry /usr/local/bin/
```

---

## 일회성 설정

아래 단계를 **순서대로** 진행합니다. 각 단계는 이전 단계에 의존하므로 건너뛰지 마세요.

### 1단계. 키 쌍 생성

```bash
tesla-sentry keygen
```

`~/.config/tesla-sentry/`에 두 개의 파일을 생성합니다:

| 파일 | 권한 | 용도 |
|---|---|---|
| `private-key.pem` | 0600 | 차량 명령에 서명 (절대 외부에 노출 금지) |
| `public-key.pem` | 0644 | 공개적으로 호스팅하여 테슬라가 명령을 검증 |

또한 호스팅에 필요한 경로를 출력합니다:

```
Host the PUBLIC key at: https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
```

---

### 2단계. (참고) Cloudflare가 무엇이고 왜 필요한가

테슬라는 보안상 "이 앱의 주인이 특정 도메인을 실제로 소유하고 있는지"를 검증합니다. 검증 방식은 **공개키 파일(`public-key.pem`)을 인터넷에 HTTPS로 띄워 두면, 테슬라가 그 파일을 직접 가져가 확인**하는 것입니다.

따라서 공개키 파일 하나를 HTTPS로 호스팅할 수단이 필요한데, 도메인을 돈 주고 살 필요 없이 **Cloudflare Pages**(정적 파일을 무료로 HTTPS 호스팅해주는 서비스)가 주는 무료 주소 `xxx.pages.dev`를 쓰면 됩니다.

올려야 할 정확한 위치는 다음과 같습니다(이 경로가 정확히 맞아야 함):

```
https://xxx.pages.dev/.well-known/appspecific/com.tesla.3p.public-key.pem
```

---

### 3단계. Cloudflare Pages에 공개키 배포

#### 3-1. 계정 만들기
[dash.cloudflare.com/sign-up](https://dash.cloudflare.com/sign-up) 에서 이메일로 가입합니다 (무료, 카드 등록 불필요).

#### 3-2. 업로드할 폴더를 PC에서 먼저 구성
경로 구조가 정확해야 하므로, 터미널에서 다음과 같이 폴더를 만듭니다:

```bash
mkdir -p ~/tesla-pages/.well-known/appspecific
cp ~/.config/tesla-sentry/public-key.pem \
   ~/tesla-pages/.well-known/appspecific/com.tesla.3p.public-key.pem
```

이제 `~/tesla-pages/` 안에 `.well-known/appspecific/com.tesla.3p.public-key.pem` 구조가 생깁니다.

> 먼저 **가입한 이메일을 인증**하세요. 인증 전에는 Pages 프로젝트 생성이 `Your user email must been verified [code: 8000077]` 오류로 막힙니다. 받은편지함의 Cloudflare 인증 메일 링크를 누르거나, 대시보드 상단 배너에서 재전송하세요.

#### 3-3. Pages 프로젝트 생성 + 업로드 (Wrangler CLI — 권장)

웹 대시보드 드래그 업로드는 `.well-known`처럼 점(`.`)으로 시작하는 폴더를 자주 빠뜨립니다(3-5 경고 참고). 우리가 올릴 파일이 정확히 그 점폴더 안에 있으므로, 점폴더를 확실히 포함하는 **Wrangler CLI를 권장**합니다.

```bash
# 1) 로그인 — 브라우저가 열리면 3-1 계정으로 승인(Allow)
npx wrangler login

# 2) 프로젝트 생성 (이름이 곧 `이름.pages.dev` 도메인이 됨)
npx wrangler pages project create 이름 --production-branch=main

# 3) 배포 — 반드시 --branch=main 으로 production에 올림
npx wrangler pages deploy ~/tesla-pages --project-name=이름 --branch=main
```

> ⚠️ **`--branch` 주의 (가장 흔한 함정)** — Wrangler는 현재 git 브랜치 이름으로 배포 환경을 정합니다. 기능 브랜치(예: `feat/...`)에서 그냥 배포하면 production이 아니라 `브랜치명.이름.pages.dev` 같은 **미리보기 주소**로 올라가고, 정작 `이름.pages.dev`는 404가 납니다. 테슬라 등록·`config.json`은 production 주소(`이름.pages.dev`)를 쓰므로, **`--branch=main`을 명시**해 production으로 올리세요.
>
> 그 외 자주 보는 오류: `Project not found` → 2)의 프로젝트 생성을 먼저 해야 합니다. git 미커밋 경고가 거슬리면 `--commit-dirty=true`를 추가하세요.

<details>
<summary>대안: 웹 대시보드 드래그 업로드</summary>

1. Cloudflare 대시보드 왼쪽 메뉴 → **Workers & Pages**
2. **Create** → **Pages** 탭 → **Upload assets** (Direct Upload) 선택
3. 프로젝트 이름 입력 (이 이름이 곧 `이름.pages.dev` 도메인이 됨) → **Create project**
4. `~/tesla-pages/` **폴더를 통째로 드래그**해 업로드 → **Deploy site**

이 방법은 점폴더 누락 위험이 있으므로 3-5의 `curl` 검증을 반드시 통과하는지 확인하세요.
</details>

#### 3-4. 도메인 확인
배포가 끝나면 `https://이름.pages.dev` 주소가 나옵니다. 이게 이후 단계의 `xxx.pages.dev`이며, 뒤에서 작성할 `config.json`의 `"domain"` 값에도 이 값(`이름.pages.dev`)을 넣습니다.

#### 3-5. 제대로 올라갔는지 검증 (가장 중요)
```bash
curl https://이름.pages.dev/.well-known/appspecific/com.tesla.3p.public-key.pem
```
→ `-----BEGIN PUBLIC KEY-----` 로 시작하는 내용이 그대로 출력되면 **성공**입니다.

> ⚠️ **`.well-known` 점(.)폴더 주의** — Cloudflare Pages가 가끔 이름이 점(`.`)으로 시작하는 폴더를 업로드에서 빼먹습니다. 위 `curl`에서 **404가 나오면** 이 문제입니다. 다음 대안 중 하나로 해결하세요:
> - **Wrangler CLI 사용** (점폴더를 확실히 포함):
>   ```bash
>   npx wrangler pages deploy ~/tesla-pages --project-name=이름
>   ```
> - 또는 해당 폴더를 GitHub 저장소에 올리고 Pages를 Git 연동으로 배포

---

### 4단계. 테슬라 개발자 앱 생성

[developer.tesla.com](https://developer.tesla.com)에 접속해 다음 설정으로 애플리케이션을 생성합니다:

| 설정 | 값 |
|---|---|
| Allowed Origin | `https://xxx.pages.dev` |
| Redirect URI | `https://xxx.pages.dev/callback` |
| Scopes | `vehicle_device_data` + `vehicle_cmds` |

앱 생성 후 **Client ID**와 **Client Secret**을 복사합니다. (한글 UI에서는 각각 "고객ID", "고객비밀번호"로 표시됩니다. Secret은 생성 직후 한 번만 보이는 경우가 있으니 바로 복사해 두세요.)

**자주 막히는 지점**

- **앱 이름에 "Tesla"를 넣으면 거부됩니다** (상표 보호). 이름은 단순 라벨일 뿐 도메인/도구와 무관하니 `Sentry Scheduler` 같은 이름을 쓰세요.
- 화면에 **"법인차량 API / Fleet API"**라고 표시돼도 정상입니다. 테슬라 API의 공식 명칭이 "Fleet API"일 뿐, 개인이 자기 차 1대를 쓰는 경우도 동일한 API를 사용합니다.
- **Scopes 체크박스 ↔ 실제 scope 매핑** — 우리 도구는 `차량정보`(`vehicle_device_data`)와 `차량명령`(`vehicle_cmds`)만 있으면 됩니다. 나머지(프로필정보/차량위치/차량충전관리/에너지제품정보·명령)는 불필요합니다. `openid`·`offline_access`는 이 체크박스 목록에 없고 표준 OAuth scope로 자동 부여되므로 따로 챙길 필요가 없습니다.

---

### 5단계. config.json 작성

```bash
cat > ~/.config/tesla-sentry/config.json << 'EOF'
{
  "client_id":     "YOUR_CLIENT_ID",
  "client_secret": "YOUR_CLIENT_SECRET",
  "vin":           "YOUR_17_CHAR_VIN",
  "domain":        "xxx.pages.dev",
  "region":        "na"
}
EOF
chmod 600 ~/.config/tesla-sentry/config.json
```

- `client_id`, `client_secret` — 4단계에서 복사한 값
- `vin` — 차량 17자리 VIN (테슬라 앱 또는 차량에서 확인)
- `domain` — 3단계에서 확보한 `이름.pages.dev` (앞에 `https://` 없이)
- `region` — 북미/APAC(한국 포함)는 반드시 `na`. Fleet API 기본 URL은 `https://fleet-api.prd.na.vn.cloud.tesla.com`입니다.

> 설정 디렉터리는 `XDG_CONFIG_HOME`을 따르며 기본값은 `~/.config/tesla-sentry/`입니다.

---

### 6단계. 테슬라에 도메인 등록

```bash
tesla-sentry register
```

파트너 토큰을 발급받아 Fleet API를 호출하여 `xxx.pages.dev` 도메인을 등록합니다. 테슬라는 이때 3단계에서 올린 공개키 파일을 가져가 도메인 소유를 검증합니다. (3-5의 `curl` 검증이 통과했어야 이 단계가 성공합니다.)

---

### 7단계. 로그인 (사용자 OAuth)

```bash
tesla-sentry login
```

명령이 인증 URL을 출력합니다:

```
1. Open this URL in a browser and approve:
   https://auth.tesla.com/oauth2/v3/authorize?...
2. After redirect, copy the `code` query parameter from the URL bar.
Paste code:
```

URL을 브라우저에서 열어 요청된 권한을 승인하고, 테슬라가 `https://xxx.pages.dev/callback?code=...`로 리디렉트하면 URL 표시줄에서 `code` 값을 복사해 프롬프트에 붙여넣습니다.

> 리디렉트된 페이지가 **"페이지를 찾을 수 없음"(404)**으로 보이는 것은 **정상**입니다. 우리는 `/callback` 페이지를 두지 않으며, 주소창의 `code` 값만 필요합니다. URL이 `...?code=값&issuer=...&state=...` 형태라면 **`code=` 와 그다음 `&` 사이의 값만** 복사하세요. 인증 코드는 몇 분 내 만료되므로 바로 붙여넣고, 늦었으면 `tesla-sentry login`을 다시 실행하세요.

토큰은 `~/.config/tesla-sentry/token.json`(0600)에 저장됩니다.

---

### 8단계. 테슬라 앱에서 virtual key 등록

휴대폰에서 다음 주소를 엽니다 (`xxx.pages.dev`는 본인 도메인으로 교체):

```
https://tesla.com/_ak/xxx.pages.dev
```

테슬라 앱이 이 애플리케이션의 virtual key를 추가할지 묻습니다. **수락**하세요. 이 단계를 거치지 않으면 서명된 차량 명령이 거부됩니다.

---

### 9단계. 설정 확인

```bash
tesla-sentry status
```

예상 출력:

```
sentry mode: false
```

(감시모드가 이미 켜져 있으면 `true`). 이 시점에 오류가 나면 위 단계 중 빠진 것이 있다는 뜻입니다.

---

## 사용법

설정이 끝나면 다음 명령으로 직접 켜고 끌 수 있습니다:

```bash
tesla-sentry on      # 감시모드 켜기
tesla-sentry off     # 감시모드 끄기
tesla-sentry status  # 현재 상태 확인

tesla-sentry afterblow            # 애프터블로우 (기본 8분)
tesla-sentry afterblow 3          # 3분만 건조
tesla-sentry afterblow 3 vent     # 3분 + 건조 중 창문 환기
```

---

## 감시모드 스케줄 (PWA + PC crontab)

감시모드 ON/OFF 시각은 **휴대폰 PWA 앱**의 "감시 모드(Sentry)" 섹션에서 설정합니다. 시각은 Cloudflare KV에 저장되어 기기 간 자동으로 동기화됩니다.

> **아키텍처 변경 (2026-06):** Cloudflare Cron Trigger 플랫폼 장애로 `scheduled()` 핸들러가 실행되지 않는 문제가 발생했습니다. Worker·KV·PWA는 변경 없이 그대로 유지하고, **타이머 역할만 PC crontab으로 이전**했습니다. Worker의 cron 트리거는 이중 실행 방지를 위해 비활성화(`crons = []`)했습니다.

### 동작 구조

```
[휴대폰] PWA 앱 "감시 모드(Sentry)" 섹션 — ON 시각, OFF 시각, 활성화 토글 설정
   │  (HTTP PUT → Cloudflare Worker API)
   ▼
[Cloudflare KV] 스케줄 저장 (단일 진실 공급원, 기기 간 자동 동기화)
   │  (HTTP GET ← PC가 매분 조회)
   ▼
[PC crontab — 매분 실행] sentry-schedule-check.sh
   │  KV GET → 현재 KST 시각이 ON/OFF 시각과 "정확히 같은 분"인지 판정
   ▼
[PC] tesla-sentry on / off 직접 실행
```

시각은 **KST(한국 표준시)** 기준입니다. PC가 KST이므로 별도 시간대 변환은 불필요합니다.

**정시(==) 판정:** crontab이 매분 실행하지만, 현재 시각이 ON/OFF 시각과 *정확히 같은 분*일 때만 실행하므로 상태 파일 없이 하루 1회 동작합니다. 트레이드오프로, 그 1분에 PC가 꺼져 있으면 그날은 건너뜁니다(PC가 상시 켜져 있으면 사실상 문제없음).

### PC crontab 등록

```bash
( crontab -l 2>/dev/null; echo '* * * * * /home/allen/Projects/tesla/scripts/sentry-schedule-check.sh' ) | crontab -
```

- 로그: `~/.config/tesla-sentry/sentry.log`
- 테스트 모드 (실제 명령 실행 없음): `SENTRY_DRY_RUN=1 /path/to/sentry-schedule-check.sh`

> crontab 줄의 시각은 `* * * * *` (매분)로 고정합니다 — ON/OFF 시각은 KV에서 가져오므로 crontab을 직접 편집하지 않아도 됩니다. 폰에서 시각을 바꾸면 즉시 반영됩니다.

### Worker 배포

Worker 배포 절차(KV 네임스페이스 생성, `wrangler secret put SENTRY_TOKEN`, `wrangler deploy`)는 **`worker/README.md`** 를 참고하세요. 배포 후 출력되는 Worker URL과 `SENTRY_TOKEN` 값을 `webapp/app.js`의 `SENTRY_API` · `SENTRY_TOKEN` 상수에 기입합니다. `wrangler.toml`의 `crons = []`은 현재 비활성 상태입니다(Cloudflare cron 복구 시 재활성화 예정).

### 수동 제어

스케줄과 무관하게 즉시 켜거나 끄려면 CLI를 직접 실행합니다:

```bash
tesla-sentry on      # 감시모드 즉시 켜기
tesla-sentry off     # 감시모드 즉시 끄기
tesla-sentry status  # 현재 상태 확인
```

---

## 애프터블로우 자동화

주차 직후 에어컨 증발기에 남은 습기를 말려 곰팡이·냄새를 예방하는 기능입니다. **차량을 깨워 폴링하는 비용/배터리 부담 없이**, 주차 후 휴대폰 홈 화면의 PWA 앱 버튼 한 번으로 트리거합니다.

### 동작 구조

```
[휴대폰] 건조 시작 버튼 탭
   │  (PWA 앱: HTTP POST)
   ▼
[ntfy.sh] 무료 pub/sub 중계
   │  (PC가 outbound로 구독 — 공인 IP 불필요)
   ▼
[PC] systemd 데몬 → afterblow 명령 → [차량] 맥스 디포스트로 N분 건조
```

PC는 **바깥으로 나가는(outbound) 구독 연결만** 사용하므로 공인 IP·포트개방·도메인이 필요 없습니다.

### 건조 방식 (왜 "맥스 디포스트"인가)

Fleet API에는 "에어컨 끄고 송풍만"을 임의 조건에서 켜는 명령이 없고, 사용하는 SDK(`vehicle-command`)는 **온도를 Hi/Lo로만** 설정할 수 있어 임의 온도(예: 22°C) 복원이 불가능합니다. 그래서 `afterblow`는 **맥스 디포스트(`SetPreconditioningMax`)** 를 사용합니다 — 끄면 이전 공조 상태로 **자동 복귀**하므로 설정온도가 "Hi"로 남지 않습니다.

> 참고: 온도와 팬 세기는 Fleet API/SDK가 노출하지 않아 **조절 불가**합니다. 조절 가능한 값은 **건조 시간**(과 창문 환기 on/off)뿐입니다.

### 설치

#### 1. 휴대폰 (PWA 앱)

`webapp/`에 있는 단일 화면 웹앱을 Cloudflare Pages에 배포합니다. **공개키용 Pages 프로젝트와는 별개의 새 프로젝트**(`tesla-afterblow`)로 올려, 기존 공개키 배포를 덮어쓰지 않게 합니다.

```bash
# (최초 1회) Cloudflare 로그인 — 3단계에서 이미 했으면 생략
npx wrangler login

# (최초 1회) PWA 전용 Pages 프로젝트 생성
npx wrangler pages project create tesla-afterblow --production-branch=main

# 배포 (앱을 수정할 때마다 이 명령만 다시 실행하면 두 폰에 즉시 반영)
npx wrangler pages deploy webapp --project-name=tesla-afterblow --branch=main
```

배포 후 URL: `https://tesla-afterblow.pages.dev`

> ⚠️ 3단계와 동일하게 **`--branch=main`을 명시**해 production으로 올리세요(기능 브랜치에서 빼먹으면 미리보기 주소로 올라가 `tesla-afterblow.pages.dev`가 404).

휴대폰 Chrome에서 이 URL을 열고 **메뉴 → "홈 화면에 추가"** 를 눌러 홈 화면 아이콘을 등록하세요. 이후에는 아이콘 탭 한 번으로 앱이 열립니다.

**앱 화면 구성:**

| UI 요소 | 설명 |
|---|---|
| 건조 시간 슬라이더 | 1–3분 선택, 기본 3분 |
| 창문 살짝 열기 토글 | 기본 ON — 건조 중 창문을 살짝 열어 습한 공기 배출 (보안·비 주의) |
| 건조 시작 버튼 | `afterblow <분> [vent]` 형식으로 ntfy 토픽에 POST |

> 토픽 URL(`https://ntfy.sh/tesla-ab-9f3k7q2zx8m`)은 `webapp/app.js`와 `scripts/afterblow-listener.sh` 두 곳에 하드코딩되어 있습니다. 다른 토픽을 쓰려면 두 파일 모두 변경하세요. 토픽 이름은 공개 URL이므로 **추측 어려운 랜덤 문자열**로 유지하세요.

#### 2. PC (구독 데몬)

`scripts/`의 스크립트와 서비스 파일을 사용합니다.

**먼저 `scripts/tesla-afterblow.service`를 본인 환경에 맞게 수정하세요.** 기본값은 `allen` 사용자와 `/home/allen/Projects/tesla` 경로로 하드코딩돼 있으므로, 다른 PC에서는 세 줄을 바꿔야 합니다:

```ini
User=<본인-리눅스-사용자>
Environment=HOME=/home/<본인-리눅스-사용자>     # tesla-sentry가 ~/.config/tesla-sentry 를 찾는 데 필요
ExecStart=<저장소-절대경로>/scripts/afterblow-listener.sh
```

그 다음 설치합니다:

```bash
chmod +x scripts/afterblow-listener.sh scripts/afterblow-run.sh
sudo cp scripts/tesla-afterblow.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now tesla-afterblow
systemctl status tesla-afterblow --no-pager   # active (running) 확인
```

> 데몬은 `<저장소>/afterblow.log`에 로그를 기록합니다(git 무시 대상). 서비스 로그는 `journalctl -u tesla-afterblow -f`로도 볼 수 있습니다.

| 파일 | 역할 |
|---|---|
| `scripts/afterblow-lib.sh` | 메시지 파싱 공통 함수 (`listener`·`run`이 함께 사용) |
| `scripts/afterblow-listener.sh` | ntfy 토픽 상시 구독(자동 재접속) → 메시지 수신 시 핸들러 호출 |
| `scripts/afterblow-run.sh` | 메시지의 분/환기로 `tesla-sentry afterblow` 실행 |
| `scripts/tesla-afterblow.service` | 부팅 자동시작 + 죽으면 자동 재시작 (`HOME` 지정 — 설정 디렉터리 탐색에 필요) |

> 토픽 URL은 `afterblow-listener.sh` 상단의 `TOPIC_URL`을, 서비스의 사용자/경로는 `tesla-afterblow.service`를 본인 환경에 맞게 수정하세요.

### 메시지 형식 및 설정값

**메시지 형식:** `afterblow <분> [vent]`

| 예시 | 동작 |
|---|---|
| `afterblow 2` | 2분 건조, 창문 닫힘 |
| `afterblow 3 vent` | 3분 건조 + 창문 살짝 열기 |
| `afterblow` (레거시) | 3분 건조, 창문 닫힘 (하위 호환) |

분은 1–3으로 클램프됩니다. PWA 앱은 트리거할 때마다 슬라이더·토글 값을 메시지에 담아 전송하므로, 건조 시간·창문 환기는 스크립트에 하드코딩하지 않습니다.

> 참고: 기본 분이 두 곳에서 다릅니다 — **ntfy 경로**(bash 파서)에서 분을 생략하면 3분이고, **`tesla-sentry afterblow`를 직접 호출**할 때 분을 생략하면 8분(Go CLI 기본값)입니다. PWA 앱은 항상 분을 명시해 보내므로 이 차이의 영향을 받지 않습니다.

**환경변수로 덮어쓸 수 있는 값 (`scripts/afterblow-run.sh`)**

| 변수 | 기본값 | 설명 |
|---|---|---|
| `DEBOUNCE` | `60` | 이 시간(초) 안의 재트리거는 무시 — 버튼을 연속으로 눌러도 중복 실행 방지. |

스크립트 수정은 데몬 재시작 없이 다음 트리거부터 자동 반영됩니다(핸들러를 매번 새로 실행하므로). 단, `tesla-afterblow.service` 파일을 바꾼 경우에만 `sudo systemctl daemon-reload && sudo systemctl restart tesla-afterblow`가 필요합니다.

### 테스트

```bash
# 전체 경로(휴대폰 없이) 검증: 수동으로 메시지 발사
curl -d 'afterblow 1' https://ntfy.sh/tesla-ab-9f3k7q2zx8m
tail -f afterblow.log                                  # TRIGGERED → command finished 확인

# 단위 테스트 (bash 파서 + JS 메시지 빌더)
for t in scripts/test-afterblow-*.sh; do bash "$t"; done
node webapp/message.test.mjs
```

차량 동작까지 확인하려면 차를 보면서 짧게 실행하는 것을 권장합니다: `tesla-sentry afterblow 1`

---

## 문제 해결

**토큰 만료** — `tesla-sentry`는 access token을 자동으로 갱신하지만, refresh token 자체가 만료되면(보통 90일간 미사용 시) 인증 오류가 발생합니다. 해결: `tesla-sentry login`을 다시 실행하세요.

**`vehicle offline`** — `on`/`off` 명령은 wake 신호를 보내고 차량이 온라인이 될 때까지 재시도합니다. 최대 3분의 명령 타임아웃까지 걸릴 수 있습니다. 별다른 조치는 필요 없으며, 보통 그 시간 안에 차량이 응답합니다.

**`tesla-sentry status`는 차량이 온라인이어야 합니다** — `on`/`off`(wake 신호를 보내고 재시도함)와 달리 `status`는 차량을 깨우지 않습니다. 차량이 잠들어 있으면 "vehicle offline" 또는 HTTP 408 오류가 표시됩니다. 테슬라 앱으로 차량을 깨우거나 `tesla-sentry on`/`off`를 먼저 실행한 뒤 상태를 확인하세요.

**`register` 실패** — 3-5의 `curl` 검증을 먼저 통과해야 합니다. 공개키 파일이 정확한 경로에서 HTTPS로 응답하지 않으면 도메인 등록이 실패합니다(`.well-known` 점폴더 문제 주의).

**Refresh token 회전** — 테슬라는 사용할 때마다 refresh token을 회전시킵니다. 새 토큰은 `token.json`에 자동으로 다시 기록됩니다. 예전 `token.json`을 복사하거나 복원하지 마세요 — 현재 세션이 무효화됩니다.
