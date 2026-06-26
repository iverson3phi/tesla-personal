package tesla

import (
	"context"

	"tesla-sentry/internal/config"
	"tesla-sentry/internal/oauth"
)

// ValidAccessToken returns a non-expired access token, refreshing and
// persisting the rotated refresh token when needed.
func ValidAccessToken(ctx context.Context, e oauth.Endpoints, cfg *config.Config, tok *config.Token, now int64, save func(*config.Token) error) (string, error) {
	if tok.AccessToken != "" && !tok.Expired(now) {
		return tok.AccessToken, nil
	}
	tr, err := e.Refresh(ctx, cfg.ClientID, tok.RefreshToken)
	if err != nil {
		return "", err
	}
	newTok := &config.Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    now + tr.ExpiresIn,
	}
	if newTok.RefreshToken == "" { // some responses omit a new refresh token
		newTok.RefreshToken = tok.RefreshToken
	}
	if err := save(newTok); err != nil {
		return "", err
	}
	return newTok.AccessToken, nil
}
