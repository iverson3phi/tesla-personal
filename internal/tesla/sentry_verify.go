package tesla

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// VerifyPolicy는 감시모드 설정 후 상태확인·재시도 정책이다.
type VerifyPolicy struct {
	Attempts    int           // 설정 명령 최대 시도 횟수(>=1)
	SettleDelay time.Duration // 명령 성공 후 첫 상태 읽기 전 대기(차량 반영 지연)
	Reads       int           // 시도당 상태 읽기 횟수(반영 지연 흡수, >=1)
	ReadDelay   time.Duration // 상태 읽기 사이 대기
	RetryDelay  time.Duration // 실패/불일치 후 다음 시도 전 대기
}

// DefaultVerifyPolicy: 최대 3회 시도, 시도당 최대 3회 확인(5초 간격).
// 최악의 경우 대략 3×(깨우기+15초)+2×20초 — commandTimeout 안에서 끝나도록 잡았다.
var DefaultVerifyPolicy = VerifyPolicy{
	Attempts:    3,
	SettleDelay: 5 * time.Second,
	Reads:       3,
	ReadDelay:   5 * time.Second,
	RetryDelay:  20 * time.Second,
}

// SetVerified는 set(want)를 실행한 뒤 get으로 실제 상태가 want가 됐는지 확인하고,
// 명령 실패나 상태 불일치면 policy에 따라 재시도한다. 반환값은 마지막으로 관측한
// 감시모드 상태(관측 실패 시 !want)와, want에 도달하지 못했을 때의 에러다.
// logf가 nil이 아니면 매 상태 읽기 결과와 실패 경과를 기록한다.
func SetVerified(
	ctx context.Context,
	want bool,
	p VerifyPolicy,
	set func(ctx context.Context, on bool) error,
	get func(ctx context.Context) (bool, error),
	logf func(format string, args ...any),
) (bool, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	attempts := max(p.Attempts, 1)
	reads := max(p.Reads, 1)
	last := !want
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			if err := sleepCtx(ctx, p.RetryDelay); err != nil {
				return last, err
			}
		}

		if err := set(ctx, want); err != nil {
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			lastErr = fmt.Errorf("attempt %d set: %w", attempt, err)
			logf("sentry verify: attempt %d/%d set failed: %v", attempt, attempts, err)
			continue
		}

		if err := sleepCtx(ctx, p.SettleDelay); err != nil {
			return last, err
		}
		for r := 1; r <= reads; r++ {
			if r > 1 {
				if err := sleepCtx(ctx, p.ReadDelay); err != nil {
					return last, err
				}
			}
			on, err := get(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return last, ctx.Err()
				}
				lastErr = fmt.Errorf("attempt %d read: %w", attempt, err)
				logf("sentry verify: attempt %d/%d read %d failed: %v", attempt, attempts, r, err)
				continue
			}
			last = on
			logf("sentry verify: attempt %d/%d read %d/%d: sentry=%s (want %s)",
				attempt, attempts, r, reads, onOff(on), onOff(want))
			if on == want {
				return on, nil
			}
			lastErr = fmt.Errorf("attempt %d: sentry is %v, want %v", attempt, on, want)
		}
		logf("sentry verify: attempt %d/%d not confirmed: %v", attempt, attempts, lastErr)
	}
	return last, errors.Join(fmt.Errorf("sentry not %v after %d attempts", want, attempts), lastErr)
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
