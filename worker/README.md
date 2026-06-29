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
