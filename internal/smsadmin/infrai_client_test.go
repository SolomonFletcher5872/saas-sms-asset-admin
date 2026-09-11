package smsadmin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateSMSAssetRetriesAndPreservesIdempotencyKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sms/template/create" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "tenant-a:approve-7" {
			t.Fatalf("Idempotency-Key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"tpl-7"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	client.Sleep = func(context.Context, time.Duration) error { return nil }
	command := CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleActive, Admin: true, Kind: AssetTemplate, OperationID: "approve-7"}
	data, err := client.CreateSMSAsset(context.Background(), command, json.RawMessage(`{"name":"welcome","body":"Hello {{name}}","variables":{"name":"Ada"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || string(data) != `{"id":"tpl-7"}` {
		t.Fatalf("requests = %d, data = %s", requests, data)
	}
}

func TestCreateSMSAssetRejectsInvalidCapabilityDocument(t *testing.T) {
	client := NewClient("test-key")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid document reached the network")
		return nil, nil
	})}
	base := CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleActive, Admin: true, OperationID: "approve-8"}
	tests := []struct {
		name     string
		kind     AssetKind
		document string
	}{
		{"signature unknown field", AssetSignature, `{"name":"Example","body":"no"}`},
		{"template unknown field", AssetTemplate, `{"name":"welcome","body":"Hello","template_vars":{}}`},
		{"signature missing name", AssetSignature, `{}`},
		{"template missing body", AssetTemplate, `{"name":"welcome"}`},
		{"document is not object", AssetTemplate, `[]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := base
			command.Kind = tt.kind
			if _, err := client.CreateSMSAsset(context.Background(), command, json.RawMessage(tt.document)); err == nil {
				t.Fatal("CreateSMSAsset() error = nil")
			}
		})
	}
}

func TestValidateDocumentAcceptsContractFields(t *testing.T) {
	tests := []struct {
		name     string
		kind     AssetKind
		document string
	}{
		{"signature", AssetSignature, `{"name":"Example","type":"company","proof_url":"https://example.com/proof","remark":"approved","vendor":"vendor-a"}`},
		{"template", AssetTemplate, `{"name":"welcome","body":"Hello","locale":"en","variables":{"name":"Ada"},"vendor":"vendor-a","message_type":"transactional","international":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDocument(tt.kind, json.RawMessage(tt.document)); err != nil {
				t.Fatalf("validateDocument() error = %v", err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
