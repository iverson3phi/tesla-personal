// Package oauth performs Tesla Fleet OAuth token operations.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPClient is overridable in tests.
var HTTPClient = http.DefaultClient

// Endpoints holds region-specific OAuth URLs.
type Endpoints struct {
	TokenURL      string
	AuthorizeBase string
	Audience      string
}

// NA returns the North America endpoints (verbatim Global Constraints).
func NA() Endpoints {
	return Endpoints{
		TokenURL:      "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token",
		AuthorizeBase: "https://auth.tesla.com/oauth2/v3/authorize",
		Audience:      "https://fleet-api.prd.na.vn.cloud.tesla.com",
	}
}

// TokenResponse is the subset of the token endpoint reply we use.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (e Endpoints) post(ctx context.Context, form url.Values) (*TokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tr, nil
}

// Refresh exchanges a refresh token for a new access (and rotated refresh) token.
// It does NOT send client_secret per the Tesla Fleet API spec.
func (e Endpoints) Refresh(ctx context.Context, clientID, refreshToken string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	})
}

// Exchange swaps an authorization code for tokens.
func (e Endpoints) Exchange(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"audience":      {e.Audience},
		"redirect_uri":  {redirectURI},
	})
}

// PartnerToken gets a client_credentials token for partner-account calls.
func (e Endpoints) PartnerToken(ctx context.Context, clientID, clientSecret, scope string) (*TokenResponse, error) {
	return e.post(ctx, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {scope},
		"audience":      {e.Audience},
	})
}

// AuthorizeURL builds the user-facing consent URL.
func (e Endpoints) AuthorizeURL(clientID, redirectURI, scope, state string) string {
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"scope":         {scope},
		"state":         {state},
	}
	return e.AuthorizeBase + "?" + q.Encode()
}
