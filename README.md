# tesla-sentry

Tesla Fleet API를 사용해 정해진 일정에 따라 테슬라 감시모드(Sentry Mode)를 자동으로 켜고 끕니다.

## 사전 준비물

- **Go 1.23 이상** — `go version`이 `go1.23` 이상을 출력해야 합니다
- **테슬라 계정** — 계정에 차량이 등록되어 있어야 합니다
- **Cloudflare Pages 사이트** (무료 플랜) — 공개키를 안정적인 `https://xxx.pages.dev` URL로 호스팅하는 데 사용합니다

## 빌드

```bash
go build -o tesla-sentry ./cmd/tesla-sentry
sudo install tesla-sentry /usr/local/bin/
```

## 일회성 설정

아래 단계를 순서대로 진행합니다. 각 단계는 이전 단계에 의존합니다.

### 1. 키 쌍 생성

```bash
tesla-sentry keygen
```

`~/.config/tesla-sentry/`에 두 개의 파일을 생성합니다:

| 파일 | 권한 | 용도 |
|---|---|---|
| `private-key.pem` | 0600 | 차량 명령에 서명 |
| `public-key.pem` | 0644 | 공개적으로 호스팅하여 테슬라가 명령을 검증 |

또한 호스팅에 필요한 경로를 출력합니다:

```
Host the PUBLIC key at: https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
```

### 2. Cloudflare Pages에 공개키 배포

`~/.config/tesla-sentry/public-key.pem`을 Cloudflare Pages 프로젝트의 정확히 다음 경로에 업로드합니다:

```
/.well-known/appspecific/com.tesla.3p.public-key.pem
```

Pages 도메인을 기록해 두세요 — `xxx.pages.dev` 형태입니다. 이후 모든 단계에서 사용합니다.

파일이 접근 가능한지 확인합니다:

```bash
curl https://xxx.pages.dev/.well-known/appspecific/com.tesla.3p.public-key.pem
```

### 3. 테슬라 개발자 앱 생성

[developer.tesla.com](https://developer.tesla.com)에 접속해 다음 설정으로 애플리케이션을 생성합니다:

| 설정 | 값 |
|---|---|
| Allowed Origin | `https://xxx.pages.dev` |
| Redirect URI | `https://xxx.pages.dev/callback` |
| Scopes | `vehicle_device_data vehicle_cmds` |

앱 생성 후 **Client ID**와 **Client Secret**을 복사합니다.

### 4. config.json 작성

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

> `region`은 북미/APAC(한국 포함)의 경우 반드시 `na`여야 합니다. Fleet API 기본 URL은
> `https://fleet-api.prd.na.vn.cloud.tesla.com`입니다.

설정 디렉터리는 `XDG_CONFIG_HOME`을 따르며, 기본값은 `~/.config/tesla-sentry/`입니다.

### 5. 테슬라에 도메인 등록

```bash
tesla-sentry register
```

파트너 토큰을 발급받아 Fleet API를 호출하여 `xxx.pages.dev` 도메인을 등록합니다. 테슬라는 이 정보로 당신의 공개키를 조회합니다.

### 6. 로그인 (사용자 OAuth)

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

URL을 열어 요청된 권한을 승인하고, 테슬라가 `https://xxx.pages.dev/callback?code=...`로 리디렉트하면 URL 표시줄에서 `code` 값을 복사해 프롬프트에 붙여넣습니다.

토큰은 `~/.config/tesla-sentry/token.json`(0600)에 저장됩니다.

### 7. 테슬라 앱에서 virtual key 등록

휴대폰에서 다음 주소를 엽니다:

```
https://tesla.com/_ak/xxx.pages.dev
```

테슬라 앱이 이 애플리케이션의 virtual key를 추가할지 묻습니다. 수락하세요. 이 단계를 거치지 않으면 서명된 차량 명령이 거부됩니다.

### 8. 설정 확인

```bash
tesla-sentry status
```

예상 출력:

```
sentry mode: false
```

(감시모드가 이미 켜져 있으면 `true`). 이 시점에 오류가 나면 위 단계 중 빠진 것이 있다는 뜻입니다.

## Crontab

매일 밤 22:00에 감시모드를 켜고 매일 아침 07:00에 끕니다:

```bash
crontab -e
```

다음 두 줄을 추가합니다:

```cron
0 22 * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
0 7  * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
```

시각은 cron을 실행하는 머신의 로컬 타임존 기준입니다.

## 문제 해결

**토큰 만료** — `tesla-sentry`는 access token을 자동으로 갱신하지만, refresh token 자체가 만료되면(보통 90일간 미사용 시) 인증 오류가 발생합니다. 해결: `tesla-sentry login`을 다시 실행하세요.

**`vehicle offline`** — 명령은 wake 신호를 보내고 차량이 온라인이 될 때까지 재시도합니다. 최대 3분의 명령 타임아웃까지 걸릴 수 있습니다. 별다른 조치는 필요 없으며, 보통 그 시간 안에 차량이 응답합니다.

**`tesla-sentry status`는 차량이 온라인이어야 합니다** — `on`/`off`(wake 신호를 보내고 재시도함)와 달리 `status`는 차량을 깨우지 않습니다. 차량이 잠들어 있으면 "vehicle offline" 또는 HTTP 408 오류가 표시됩니다. 테슬라 앱으로 차량을 깨우거나 `tesla-sentry on`/`off`를 먼저 실행한 뒤 상태를 확인하세요.

**Refresh token 회전** — 테슬라는 사용할 때마다 refresh token을 회전시킵니다. 새 토큰은 `token.json`에 자동으로 다시 기록됩니다. 예전 `token.json`을 복사하거나 복원하지 마세요 — 현재 세션이 무효화됩니다.
