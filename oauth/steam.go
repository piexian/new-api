package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func init() {
	Register("steam", &SteamProvider{})
}

// SteamProvider implements OAuth for Steam.
//
// Note: Steam does NOT use OAuth 2.0 for website login — it uses OpenID 2.0.
// There is no authorization code and no access token. The "ExchangeToken" step
// is therefore repurposed to verify the OpenID assertion carried back on the
// callback URL (openid.* params) and extract the SteamID. The SteamID itself
// is the only credential; user profile is fetched separately via the Steam Web
// API using a publisher/user Web API Key.
type SteamProvider struct{}

var (
	// steamOpenIDEndpoint is both the authentication redirect target and the
	// verification endpoint (openid.mode=check_authentication). Declared as a
	// package var so tests can point it at a local httptest server.
	steamOpenIDEndpoint = "https://steamcommunity.com/openid/login"
	// steamAPIBase is the Web API host. GetPlayerSummaries only needs a user key.
	steamAPIBase = "https://api.steampowered.com"
)

func (p *SteamProvider) GetName() string {
	return "Steam"
}

func (p *SteamProvider) IsEnabled() bool {
	return common.SteamOAuthEnabled
}

func (p *SteamProvider) GetProviderPrefix() string {
	return "steam_"
}

func (p *SteamProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.SteamId = providerUserID
}

func (p *SteamProvider) IsUserIDTaken(providerUserID string) bool {
	return model.IsSteamIdAlreadyTaken(providerUserID)
}

func (p *SteamProvider) FillUserByProviderID(user *model.User, providerUserID string) error {
	user.SteamId = providerUserID
	return user.FillUserBySteamId()
}

// steamParams returns a single-value map for the i18n Provider template.
func steamParams() map[string]any {
	return map[string]any{"Provider": "Steam"}
}

// ExchangeToken verifies the OpenID 2.0 positive assertion returned by Steam.
//
// The `code` argument is unused (Steam has no authorization code); all OpenID
// parameters are read from the gin context query string. On success the
// verified SteamID is returned inside OAuthToken.AccessToken (a semantic reuse
// of the OAuth2 token carrier — GetUserInfo consumes it).
func (p *SteamProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*OAuthToken, error) {
	query := c.Request.URL.Query()

	// User declined the sign-in on Steam's side.
	if query.Get("openid.mode") == "cancel" {
		return nil, &AccessDeniedError{Message: "Steam authorization was cancelled"}
	}

	// Replay the assertion with mode=check_authentication to prove authenticity.
	// We forward every openid.* param verbatim (Steam signs a subset via
	// openid.signed) and only flip the mode. Skipping this check allows an
	// attacker to forge any SteamID.
	verifyValues := buildVerificationValues(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, steamOpenIDEndpoint, strings.NewReader(verifyValues.Encode()))
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, steamParams(), err.Error())
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, steamParams(), err.Error())
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Steam responds in OpenID's key:value\n format. is_valid must be "true".
	if !isAssertionValid(string(body)) {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthTokenFailed, steamParams(), "Steam OpenID assertion verification failed")
	}

	// Extract the SteamID64 from the claimed_id: .../openid/id/<SteamID64>.
	steamID := extractSteamID(query.Get("openid.claimed_id"))
	if steamID == "" {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthUserInfoEmpty, steamParams(), "missing openid.claimed_id")
	}

	return &OAuthToken{AccessToken: steamID}, nil
}

// buildVerificationValues constructs the form body for the OpenID
// check_authentication request: every openid.* parameter from the callback
// assertion is forwarded verbatim (Steam signs a subset via openid.signed),
// and openid.mode is flipped to check_authentication. Non-openid parameters
// (e.g. state) are dropped — they are not part of the signed assertion.
func buildVerificationValues(query url.Values) url.Values {
	verifyValues := url.Values{}
	for key, vals := range query {
		if !strings.HasPrefix(key, "openid.") {
			continue
		}
		if len(vals) > 0 {
			verifyValues.Set(key, vals[0])
		}
	}
	verifyValues.Set("openid.mode", "check_authentication")
	return verifyValues
}

// isAssertionValid parses the OpenID verification response and reports whether
// Steam confirmed the assertion. The response is a newline-delimited list of
// "key:value" pairs; we look for is_valid:true.
func isAssertionValid(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "is_valid:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "is_valid:")) == "true"
		}
	}
	return false
}

// extractSteamID returns the 64-bit SteamID embedded in the claimed_id URL.
// Returns "" when the value is missing or malformed.
func extractSteamID(claimedID string) string {
	claimedID = strings.TrimSpace(claimedID)
	if claimedID == "" {
		return ""
	}
	// Strip any query/fragment before taking the trailing path segment.
	if i := strings.IndexAny(claimedID, "?#"); i >= 0 {
		claimedID = claimedID[:i]
	}
	idx := strings.LastIndex(claimedID, "/")
	id := claimedID
	if idx >= 0 {
		id = claimedID[idx+1:]
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	// SteamID64 is a 17-digit decimal number.
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}

// steamPlayerSummary maps the fields we consume from GetPlayerSummaries/v2.
type steamPlayerSummary struct {
	SteamID         string `json:"steamid"`
	PersonaName     string `json:"personaname"`
	ProfileURL      string `json:"profileurl"`
	Avatar          string `json:"avatar"`
	AvatarMedium    string `json:"avatarmedium"`
	AvatarFull      string `json:"avatarfull"`
	VisibilityState int    `json:"communityvisibilitystate"`
}

// GetUserInfo fetches the Steam profile via ISteamUser/GetPlayerSummaries/v2.
//
// The "token" only carries the verified SteamID; the actual call is
// authenticated with the server-side Steam Web API Key. Steam never returns an
// email address, so OAuthUser.Email is left empty (findOrCreateOAuthUser
// handles a missing email gracefully).
func (p *SteamProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUser, error) {
	steamID := token.AccessToken
	if steamID == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, steamParams())
	}
	if common.SteamWebAPIKey == "" {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthNotEnabled, steamParams(), "Steam Web API key is not configured")
	}

	endpoint := fmt.Sprintf(
		"%s/ISteamUser/GetPlayerSummaries/v2/?key=%s&steamids=%s",
		steamAPIBase,
		url.QueryEscape(common.SteamWebAPIKey),
		url.QueryEscape(steamID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, steamParams(), err.Error())
	}

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Steam] GetUserInfo connect error: %s", err.Error()))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthConnectFailed, steamParams(), err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		logger.LogError(ctx, fmt.Sprintf("[OAuth-Steam] GetUserInfo failed: status=%d, key_len=%d, body=%s", res.StatusCode, len(common.SteamWebAPIKey), string(errBody)))
		return nil, NewOAuthErrorWithRaw(i18n.MsgOAuthGetUserErr, steamParams(), fmt.Sprintf("steam api status %d", res.StatusCode))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Response struct {
			Players []steamPlayerSummary `json:"players"`
		} `json:"response"`
	}
	if err := common.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Response.Players) == 0 {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, steamParams())
	}
	player := resp.Response.Players[0]
	if player.SteamID == "" {
		return nil, NewOAuthError(i18n.MsgOAuthUserInfoEmpty, steamParams())
	}

	// personaname is the public display name; private profiles may still expose
	// it but other fields can be empty — fall back to the SteamID for safety.
	username := player.PersonaName
	displayName := player.PersonaName
	if displayName == "" {
		displayName = "Steam User"
	}

	return &OAuthUser{
		ProviderUserID: player.SteamID,
		Username:       username,
		DisplayName:    displayName,
		Extra: map[string]any{
			"avatar":     player.Avatar,
			"avatarfull": player.AvatarFull,
			"profileurl": player.ProfileURL,
			"steamid":    player.SteamID,
		},
	}, nil
}
