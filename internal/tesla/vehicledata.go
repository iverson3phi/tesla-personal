package tesla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SentryState returns whether Sentry Mode is currently on. The vehicle must be
// online (wake first) for fresh data.
func SentryState(ctx context.Context, accessToken, vin string) (bool, error) {
	url := fmt.Sprintf("%s/api/1/vehicles/%s/vehicle_data?endpoints=vehicle_state", BaseURL, vin)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("vehicle_data %s: %s", resp.Status, string(body))
	}
	var out struct {
		Response struct {
			VehicleState struct {
				SentryMode bool `json:"sentry_mode"`
			} `json:"vehicle_state"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("decode vehicle_data: %w", err)
	}
	return out.Response.VehicleState.SentryMode, nil
}
