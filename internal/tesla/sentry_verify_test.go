package tesla

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// 테스트용 즉시 재시도 정책(대기 없음).
var fastPolicy = VerifyPolicy{Attempts: 3, SettleDelay: 0, RetryDelay: 0, Reads: 2, ReadDelay: 0}

func TestSetVerifiedSucceedsFirstTry(t *testing.T) {
	sets, reads := 0, 0
	got, err := SetVerified(context.Background(), true, fastPolicy,
		func(ctx context.Context, on bool) error { sets++; return nil },
		func(ctx context.Context) (bool, error) { reads++; return true, nil },
		nil)
	if err != nil || !got {
		t.Fatalf("got=%v err=%v, want true nil", got, err)
	}
	if sets != 1 || reads != 1 {
		t.Fatalf("sets=%d reads=%d, want 1 1", sets, reads)
	}
}

func TestSetVerifiedRetriesOnSetError(t *testing.T) {
	sets := 0
	got, err := SetVerified(context.Background(), true, fastPolicy,
		func(ctx context.Context, on bool) error {
			sets++
			if sets < 3 {
				return errors.New("vehicle unavailable")
			}
			return nil
		},
		func(ctx context.Context) (bool, error) { return true, nil },
		nil)
	if err != nil || !got || sets != 3 {
		t.Fatalf("got=%v err=%v sets=%d, want true nil 3", got, err, sets)
	}
}

func TestSetVerifiedRetriesOnMismatch(t *testing.T) {
	sets, reads := 0, 0
	got, err := SetVerified(context.Background(), true, fastPolicy,
		func(ctx context.Context, on bool) error { sets++; return nil },
		func(ctx context.Context) (bool, error) {
			reads++
			// 첫 시도의 두 번 읽기는 모두 false, 두 번째 시도에서 true.
			return sets >= 2, nil
		},
		nil)
	if err != nil || !got {
		t.Fatalf("got=%v err=%v, want true nil", got, err)
	}
	if sets != 2 || reads != 3 {
		t.Fatalf("sets=%d reads=%d, want 2 3", sets, reads)
	}
}

func TestSetVerifiedLagThenMatchNoResend(t *testing.T) {
	sets, reads := 0, 0
	_, err := SetVerified(context.Background(), false, fastPolicy,
		func(ctx context.Context, on bool) error { sets++; return nil },
		func(ctx context.Context) (bool, error) {
			reads++
			return reads < 2, nil // 첫 읽기는 아직 이전 값(true), 둘째에 false
		},
		nil)
	if err != nil || sets != 1 {
		t.Fatalf("err=%v sets=%d, want nil 1", err, sets)
	}
}

func TestSetVerifiedGivesUpAndReportsLastState(t *testing.T) {
	sets := 0
	got, err := SetVerified(context.Background(), true, fastPolicy,
		func(ctx context.Context, on bool) error { sets++; return nil },
		func(ctx context.Context) (bool, error) { return false, nil },
		nil)
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if got {
		t.Fatal("last observed state should be false")
	}
	if sets != fastPolicy.Attempts {
		t.Fatalf("sets=%d, want %d", sets, fastPolicy.Attempts)
	}
}

func TestSetVerifiedStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	sets := 0
	p := fastPolicy
	p.Attempts = 10
	p.RetryDelay = time.Hour
	_, err := SetVerified(ctx, true, p,
		func(ctx context.Context, on bool) error { sets++; cancel(); return errors.New("boom") },
		func(ctx context.Context) (bool, error) { return false, nil },
		nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
	if sets != 1 {
		t.Fatalf("sets=%d, want 1", sets)
	}
}

func TestSetVerifiedLogsEveryRead(t *testing.T) {
	var lines []string
	_, _ = SetVerified(context.Background(), true, fastPolicy,
		func(ctx context.Context, on bool) error { return nil },
		func(ctx context.Context) (bool, error) { return true, nil },
		func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) })
	want := "sentry verify: attempt 1/3 read 1/2: sentry=on (want on)"
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("lines=%q, want [%q]", lines, want)
	}
}
