package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrAuthorizationPending indicates the user has not yet entered the pairing code.
	ErrAuthorizationPending = errors.New("authorization_pending")

	// ErrSlowDown indicates the client is polling too aggressively and must add 5s to the interval.
	ErrSlowDown = errors.New("slow_down")

	// ErrTokenExpired indicates the user code expired before authorization completed.
	ErrTokenExpired = errors.New("expired_token")

	// ErrAccessDenied indicates the user rejected the authorization request.
	ErrAccessDenied = errors.New("access_denied")
)

const (
	// GoogleDeviceCodeURL is the RFC 8628 device authorization endpoint for Google OAuth 2.0.
	GoogleDeviceCodeURL = "https://oauth2.googleapis.com/device/code"

	// GoogleTokenURL is the token exchange and polling endpoint.
	GoogleTokenURL = "https://oauth2.googleapis.com/token"
)

// DeviceCodeResponse holds the RFC 8628 pairing instructions displayed on the TV screen.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// DeviceFlowClient coordinates RFC 8628 pairing and backoff polling for headless displays.
type DeviceFlowClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

// NewDeviceFlowClient creates an authenticated client targeting Google's device endpoint.
func NewDeviceFlowClient(clientID, clientSecret string) *DeviceFlowClient {
	return &DeviceFlowClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RequestDeviceCode initiates the RFC 8628 grant and obtains a user pairing code.
func (c *DeviceFlowClient) RequestDeviceCode(ctx context.Context, scopes []string) (*DeviceCodeResponse, error) {
	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("scope", strings.Join(scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GoogleDeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var deviceResp DeviceCodeResponse
	if err := json.Unmarshal(body, &deviceResp); err != nil {
		return nil, fmt.Errorf("decoding device code payload: %w", err)
	}

	if deviceResp.Interval <= 0 {
		deviceResp.Interval = 5
	}

	return &deviceResp, nil
}

// PollToken executes a single poll attempt against the token endpoint.
func (c *DeviceFlowClient) PollToken(ctx context.Context, deviceCode string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("device_code", deviceCode)
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GoogleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token poll request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polling token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp tokenErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			switch errResp.Error {
			case "authorization_pending":
				return nil, ErrAuthorizationPending
			case "slow_down":
				return nil, ErrSlowDown
			case "expired_token":
				return nil, ErrTokenExpired
			case "access_denied":
				return nil, ErrAccessDenied
			default:
				return nil, fmt.Errorf("token error %s: %s", errResp.Error, errResp.ErrorDescription)
			}
		}
		return nil, fmt.Errorf("unexpected token endpoint status %d: %s", resp.StatusCode, string(body))
	}

	type rawTokenResponse struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}

	var raw rawTokenResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}

	return &Token{
		AccessToken:  raw.AccessToken,
		TokenType:    raw.TokenType,
		RefreshToken: raw.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}, nil
}
