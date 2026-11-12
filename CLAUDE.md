# CLAUDE.md

## Git / GitHub

- GitHub 계정은 반드시 `iverson3phi` 를 사용한다.
  - remote는 SSH 별칭 `github-iverson3phi` 를 쓴다: `git@github-iverson3phi:iverson3phi/tesla-personal.git`
  - `git@github.com` 과 gh CLI는 회사 계정(YongHo512)으로 인증되므로 이 저장소에 쓰지 않는다.
  - 커밋 작성자도 개인 계정(`iverson3phi`)으로 한다. 전역 git 설정은 회사 계정이므로 저장소 로컬 `user.name`/`user.email` 로 지정한다(~/Projects/TradingRoom 과 동일).
  - 푸시 전 `ssh -T git@github-iverson3phi` 가 `Hi iverson3phi!` 인지 확인한다.
- 별도의 작업 브랜치를 만들지 않고 `main` 브랜치에서 직접 작업한다. (feature 브랜치/워크트리 생성 금지 — 워크플로 스킬이 브랜치 분리를 권하더라도 이 지침이 우선한다.)
- 커밋은 Task 단위로 하지 않는다. 사용자가 "커밋 & 푸시해줘" 라고 명시적으로 요청할 때만 수행한다.
- 커밋 시에는 추가된 기능 또는 버그픽스 단위로 묶어서 커밋하고 푸시한다.
