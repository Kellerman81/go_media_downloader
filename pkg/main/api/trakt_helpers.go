package api

import (
	"context"
	"errors"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

var (
	errTraktNotInitialized    = errors.New("trakt provider not initialized")
	errProviderNotInitialized = errors.New("provider not initialized")
)

// traktToken returns the current Trakt token, nil when there is none.
func traktToken() *apiexternal_v2.OAuthToken {
	if provider := providers.GetTrakt(); provider != nil {
		return provider.GetCurrentToken()
	}

	return nil
}

// traktAuthURL returns the URL of the Trakt authorization dialog, "" when
// Trakt is not set up.
func traktAuthURL() string {
	provider := providers.GetTrakt()
	if provider == nil {
		return ""
	}

	urlv := provider.GetAuthorizationURL("state")
	logger.Logtype("info", 0).Str("url", urlv).Msg("Visit the URL for the auth dialog")

	return urlv
}

// traktExchangeCode exchanges an authorization code for a token, which the
// provider stores and saves.
func traktExchangeCode(ctx context.Context, code string) (*apiexternal_v2.OAuthToken, error) {
	provider := providers.GetTrakt()
	if provider == nil {
		return nil, errTraktNotInitialized
	}

	token, err := provider.ExchangeCodeForToken(ctx, code)
	if err != nil {
		logger.Logtype("error", 1).Err(err).Msg("Error getting Trakt token")
	}

	return token, err
}

// traktUserList returns the items of a Trakt user list or watchlist.
func traktUserList(ctx context.Context, username, listname, listtype string) ([]trakt.TraktUserListItem, error) {
	if username == "" || listname == "" || listtype == "" {
		return nil, logger.ErrNotFound
	}

	provider := providers.GetTrakt()
	if provider == nil {
		return nil, errTraktNotInitialized
	}

	return provider.GetTraktUserList(ctx, username, listname, listtype)
}
