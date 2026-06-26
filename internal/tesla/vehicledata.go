package tesla

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// VehicleState returns the vehicle's connectivity state ("online", "asleep",
// or "offline") from the lightweight summary endpoint. Unlike vehicle_data,
// this endpoint neither wakes the car nor returns a heavy payload, so it is
// cheap to poll while waiting for a wake to complete.
func VehicleState(ctx context.Context, accessToken, vin string) (string, error) {
	url := fmt.Sprintf("%s/api/1/vehicles/%s", BaseURL, vin)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read vehicle summary: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("vehicle summary %s: %s", resp.Status, string(body))
	}
	var out struct {
		Response struct {
			State string `json:"state"`
		} `json:"response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode vehicle summary: %w", err)
	}
	return out.Response.State, nil
}

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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("read vehicle_data: %w", err)
	}
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
