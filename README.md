# tesla-sentry

Tesla Fleet API를 사용해 정해진 일정에 따라 테슬라 감시모드(Sentry Mode)를 자동으로 켜고 끕니다. 리눅스 PC에서 바이너리를 실행하면 되고, 크론탭에 등록해 매일 자동으로 동작시킬 수 있습니다.

## 사전 준비물

- **Go 1.23 이상** — `go version`이 `go1.23` 이상을 출력해야 합니다
- **테슬라 계정** — 계정에 차량이 등록되어 있어야 합니다
- **Cloudflare 계정** (무료) — 공개키 파일 하나를 HTTPS로 호스팅하는 데 사용합니다 (아래 3단계에서 설명)

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

#### 3-3. Pages 프로젝트 생성 + 업로드
1. Cloudflare 대시보드 왼쪽 메뉴 → **Workers & Pages**
2. **Create** → **Pages** 탭 → **Upload assets** (Direct Upload) 선택
3. 프로젝트 이름 입력 (이 이름이 곧 `이름.pages.dev` 도메인이 됨) → **Create project**
4. `~/tesla-pages/` **폴더를 통째로 드래그**해 업로드 → **Deploy site**

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
| Scopes | `vehicle_device_data vehicle_cmds` |

앱 생성 후 **Client ID**와 **Client Secret**을 복사합니다.

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
```

---

## Crontab 등록

매일 밤 22:00에 감시모드를 켜고 매일 아침 07:00에 끄려면:

```bash
crontab -e
```

다음 두 줄을 추가합니다:

```cron
0 22 * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
0 7  * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
```

시각은 cron을 실행하는 머신의 로컬 타임존 기준입니다. 실행 로그는 `~/.config/tesla-sentry/sentry.log`에 쌓입니다.

---

## 문제 해결

**토큰 만료** — `tesla-sentry`는 access token을 자동으로 갱신하지만, refresh token 자체가 만료되면(보통 90일간 미사용 시) 인증 오류가 발생합니다. 해결: `tesla-sentry login`을 다시 실행하세요.

**`vehicle offline`** — `on`/`off` 명령은 wake 신호를 보내고 차량이 온라인이 될 때까지 재시도합니다. 최대 3분의 명령 타임아웃까지 걸릴 수 있습니다. 별다른 조치는 필요 없으며, 보통 그 시간 안에 차량이 응답합니다.

**`tesla-sentry status`는 차량이 온라인이어야 합니다** — `on`/`off`(wake 신호를 보내고 재시도함)와 달리 `status`는 차량을 깨우지 않습니다. 차량이 잠들어 있으면 "vehicle offline" 또는 HTTP 408 오류가 표시됩니다. 테슬라 앱으로 차량을 깨우거나 `tesla-sentry on`/`off`를 먼저 실행한 뒤 상태를 확인하세요.

**`register` 실패** — 3-5의 `curl` 검증을 먼저 통과해야 합니다. 공개키 파일이 정확한 경로에서 HTTPS로 응답하지 않으면 도메인 등록이 실패합니다(`.well-known` 점폴더 문제 주의).

**Refresh token 회전** — 테슬라는 사용할 때마다 refresh token을 회전시킵니다. 새 토큰은 `token.json`에 자동으로 다시 기록됩니다. 예전 `token.json`을 복사하거나 복원하지 마세요 — 현재 세션이 무효화됩니다.
