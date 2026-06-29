package tesla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ReportSentryState는 감시모드 상태를 Worker(KV)에 PUT으로 보고한다. state는
// "on"/"off", source는 "command"/"status". 호출측은 best-effort로 다뤄야 하며
// (실패해도 명령 자체는 성공), url/token이 비면 호출하지 않는다.
func ReportSentryState(ctx context.Context, url, token, state, source string) error {
	payload, err := json.Marshal(map[string]string{"state": state, "source": source})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("report state %s: %s", resp.Status, string(body))
	}
	return nil
}
