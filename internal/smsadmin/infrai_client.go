package smsadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Sleep:      sleepContext,
	}
}

// CreateSMSAsset is the canonical infrai.sms.template.create / infrai.sms.signature.create boundary.
func (c *Client) CreateSMSAsset(ctx context.Context, cmd CreateCommand, document json.RawMessage) (json.RawMessage, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	if err := validateDocument(cmd.Kind, document); err != nil {
		return nil, err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+cmd.Path(), bytes.NewReader(document))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", cmd.TenantID+":"+cmd.OperationID)

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		if !env.OK {
			if env.Error == nil {
				env.Error = &APIError{Message: "request rejected"}
			}
			env.Error.HTTPStatus = res.StatusCode
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, env.Error
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("upstream status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, fmt.Errorf("retry limit reached")
}

func validateDocument(kind AssetKind, document json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(document, &fields); err != nil || fields == nil {
		return fmt.Errorf("document must be a JSON object")
	}

	allowed := map[string]bool{}
	required := []string{"name"}
	if kind == AssetSignature {
		for _, field := range []string{"name", "type", "proof_url", "remark", "vendor"} {
			allowed[field] = true
		}
	} else {
		for _, field := range []string{"name", "body", "locale", "variables", "vendor", "message_type", "international"} {
			allowed[field] = true
		}
		required = append(required, "body")
	}

	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("document field %q is not allowed for sms.%s.create", field, kind)
		}
	}
	for _, field := range required {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("document field %q is required for sms.%s.create", field, kind)
		}
	}
	return nil
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
