# tesla-sentry 설계 문서

작성일: 2026-06-24

## 목표

테슬라 차량의 감시모드(Sentry Mode)를 Linux PC에서 바이너리 한 번 실행으로 on/off 한다.
크론탭에 등록해 정해진 시각에 자동으로 켜고 끄는 것이 주 용도다.

## 확정된 결정 사항

- **제어 방식**: 자체 구축 Tesla Fleet API (클라우드 원격 제어). PC가 차량과 멀리 떨어져 있어 BLE 로컬 제어는 불가.
- **리전**: 북미/APAC (한국). Base URL `fleet-api.prd.na.vn.cloud.tesla.com`.
- **공개키 호스팅**: Cloudflare Pages 무료 서브도메인(`xxx.pages.dev`) + 자동 HTTPS.
- **인터페이스**: 명시적 `sentry on` / `sentry off` (크론탭 2줄). 상태 토글 아님.
- **구현**: Go 단일 정적 바이너리 + 테슬라 공식 [`vehicle-command`](https://github.com/teslamotors/vehicle-command) SDK. 2021년 이후 차량에 필수인 명령 서명을 SDK가 네이티브 처리.

## 전체 구조

하나의 Go 바이너리 `tesla-sentry`가 서브커맨드로 두 역할을 수행한다.

일회성 설정용:
```
tesla-sentry keygen      # EC P-256 키쌍 생성
tesla-sentry register    # 파트너 계정 등록 (도메인 검증)
tesla-sentry login       # OAuth 로그인 → refresh token 저장
```

운영용 (크론탭이 호출):
```
tesla-sentry on|off      # 감시모드 설정
tesla-sentry status      # 현재 감시모드 상태 조회 (dry-run 검증용)
```

## 일회성 설정 흐름

1. `tesla-sentry keygen` → `private-key.pem` + `public-key.pem` 생성
2. Cloudflare Pages에 `public-key.pem`을 `/.well-known/appspecific/com.tesla.3p.public-key.pem` 경로로 배포 → `xxx.pages.dev` 도메인 확보
3. developer.tesla.com에서 앱 생성:
   - Allowed Origin = `https://xxx.pages.dev`
   - Redirect URI = `https://xxx.pages.dev/callback` (또는 코드 수동 붙여넣기용 경로)
   - Scope = `vehicle_device_data vehicle_cmds offline_access`
   - → `client_id` / `client_secret` 발급
4. `tesla-sentry register` → 파트너 계정 등록 API(`POST /api/1/partner_accounts`) 호출. 테슬라가 공개키 파일을 가져가 도메인 검증.
5. `tesla-sentry login` → 브라우저 로그인 → 리디렉트 코드 붙여넣기 → `refresh_token` 저장
6. 휴대폰에서 `https://tesla.com/_ak/xxx.pages.dev` 접속 → Tesla 앱에서 virtual key를 차량에 등록(서명 명령 권한 부여)

## 운영 실행 흐름 (`tesla-sentry on` 기준)

1. 인자 파싱 (`on`/`off`)
2. 설정·토큰·개인키 로드
3. 저장된 `refresh_token`으로 access token 갱신 → 갱신된 토큰을 캐시에 저장(토큰 회전 대응)
4. 차량 `wake_up` 호출 후 online 될 때까지 폴링(타임아웃·백오프 포함)
5. 공식 SDK로 차량 연결(개인키로 명령 서명) → `SetSentryMode(true/false)` 전송
6. 결과 로그 출력 + 종료코드 반환(성공 0 / 실패 ≠0)

## 설정·비밀정보 저장

`~/.config/tesla-sentry/` (모든 파일 `chmod 600`):

- `config.toml` — `client_id`, `client_secret`, `vin`, `region`(na), `domain`
- `token.json` — `access_token`, `refresh_token`, 만료시각(자동 갱신)
- `private-key.pem` — 명령 서명용 개인키

## 에러 처리 / 로깅

- 종료코드: 성공 0, 실패 ≠0 (크론 모니터링 활용)
- 로그: stdout/stderr(크론 메일 캡처) + 선택적 `~/.config/tesla-sentry/sentry.log` append
- 일시적 오류(wake 타임아웃, 5xx, 408)는 짧은 백오프로 N회 재시도
- refresh token 회전 시 새 토큰 즉시 저장해 다음 실행 인증 실패 방지

## 테스트 전략

- **단위 테스트**: 설정 파싱, 토큰 갱신(httptest 목 서버), 인자 파싱, on/off→명령 매핑
- **통합 검증**: `tesla-sentry status`로 실제 인증·연결·상태조회까지 확인(상태 변경 없는 안전한 dry-run)
- **수동 E2E**: 실제 차량에 `on`/`off` 1회씩

## 프로젝트 구조(예상)

```
tesla/
  cmd/tesla-sentry/main.go     # 서브커맨드 디스패치
  internal/config/             # 설정·토큰 캐시 로드/저장
  internal/tesla/              # 토큰 갱신, SDK 클라이언트 래핑, 명령 전송
  internal/setup/              # keygen, register, login 헬퍼
  docs/
```

## 크론탭 사용 예 (최종 결과물)

```cron
# 매일 22:00 감시모드 ON, 07:00 OFF
0 22 * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
0 7  * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
```

## 미해결/구현 시 확인 필요

- Tesla 토큰 엔드포인트의 confidential client(refresh 시 `client_secret` 필요 여부) 동작은 구현 단계에서 실제 확인.
- `SetSentryMode`가 차량 모델/연식에 따라 서명 필요 여부가 다를 수 있으나, 서명 경로를 기본으로 구축하므로 전 차량 커버.
