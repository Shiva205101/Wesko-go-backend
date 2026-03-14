package twilio

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"vesko/auth"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type VerifyProvider struct {
	accountSID string
	authToken  string
	serviceSID string
	baseURL    string
	client     HTTPDoer
}

func NewVerifyProvider(accountSID string, authToken string, serviceSID string, client HTTPDoer) *VerifyProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &VerifyProvider{
		accountSID: strings.TrimSpace(accountSID),
		authToken:  strings.TrimSpace(authToken),
		serviceSID: strings.TrimSpace(serviceSID),
		baseURL:    "https://verify.twilio.com/v2/Services",
		client:     client,
	}
}

func (p *VerifyProvider) SendVerification(ctx context.Context, mobile string) error {
	if p.accountSID == "" || p.authToken == "" || p.serviceSID == "" {
		return auth.ErrOTPProviderUnavailable
	}
	values := url.Values{}
	values.Set("To", mobile)
	values.Set("Channel", "sms")
	_, err := p.postForm(ctx, p.serviceURL("/Verifications"), values, false)
	return err
}

func (p *VerifyProvider) CheckVerification(ctx context.Context, mobile string, code string) (bool, error) {
	if p.accountSID == "" || p.authToken == "" || p.serviceSID == "" {
		return false, auth.ErrOTPProviderUnavailable
	}
	values := url.Values{}
	values.Set("To", mobile)
	values.Set("Code", strings.TrimSpace(code))
	body, err := p.postForm(ctx, p.serviceURL("/VerificationCheck"), values, true)
	if err != nil {
		return false, err
	}
	var response struct {
		Status string `json:"status"`
		Valid  bool   `json:"valid"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false, err
	}
	return response.Valid || strings.EqualFold(response.Status, "approved"), nil
}

func (p *VerifyProvider) serviceURL(path string) string {
	return p.baseURL + "/" + p.serviceSID + path
}

func (p *VerifyProvider) postForm(ctx context.Context, endpoint string, values url.Values, needBody bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.accountSID, p.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, auth.ErrOTPProviderUnavailable
		}
		return nil, auth.ErrInvalidOTPCode
	}
	if !needBody {
		return nil, nil
	}
	return body, nil
}

var _ auth.OTPProvider = (*VerifyProvider)(nil)
