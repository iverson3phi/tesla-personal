package tesla

import (
	"context"
	"testing"
	"time"
)

// AfterBlowCancel의 시그니처를 고정한다(컴파일 가드). 실제 차량 호출은
// 자격증명/네트워크가 없으면 에러를 반환하므로, 여기서는 nil이 아닌 에러로
// 빠르게 끝나는지(=함수가 존재하고 호출 가능)만 확인한다.
func TestAfterBlowCancelSignature(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// 존재하지 않는 키 경로 → 즉시 에러. 패닉/컴파일에러가 없으면 통과.
	if err := AfterBlowCancel(ctx, "AT", "VIN", "/nonexistent/key.pem"); err == nil {
		t.Fatal("expected error with bogus key path")
	}
}
