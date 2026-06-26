// Package tesla wraps Fleet API HTTP calls and the vehicle-command SDK.
package tesla

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BaseURL is the NA Fleet API base; overridable in tests.
var BaseURL = "https://fleet-api.prd.na.vn.cloud.tesla.com"

// HTTPClient is overridable in tests.
var HTTPClient = http.DefaultClient

// RegisterPartner registers the hosting domain with Tesla so vehicles can
// pair the public key. Authorized by a client_credentials partner token.
func RegisterPartner(ctx context.Context, partnerToken, domain string) error {
	body, _ := json.Marshal(map[string]string{"domain": domain})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, BaseURL+"/api/1/partner_accounts", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+partnerToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		if readErr != nil {
			return fmt.Errorf("partner_accounts %s: <response body unreadable: %v>", resp.Status, readErr)
		}
		return fmt.Errorf("partner_accounts %s: %s", resp.Status, string(rb))
	}
	return nil
}
