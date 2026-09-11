package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/saas-sms-asset-admin/internal/smsadmin"
)

type createRequest struct {
	TenantID    string             `json:"tenant_id"`
	Lifecycle   smsadmin.Lifecycle `json:"lifecycle"`
	Admin       bool               `json:"admin"`
	AssetKind   smsadmin.AssetKind `json:"asset_kind"`
	OperationID string             `json:"operation_id"`
	Document    json.RawMessage    `json:"document"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := smsadmin.NewClient(key)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/sms-assets", func(w http.ResponseWriter, r *http.Request) {
		var input createRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
			return
		}
		command := smsadmin.CreateCommand{TenantID: input.TenantID, Lifecycle: input.Lifecycle, Admin: input.Admin, Kind: input.AssetKind, OperationID: input.OperationID}
		data, err := client.CreateSMSAsset(r.Context(), command, input.Document)
		if err != nil {
			status := http.StatusBadRequest
			var apiErr *smsadmin.APIError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				status = apiErr.HTTPStatus
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]json.RawMessage{"asset": data})
	})

	address := os.Getenv("LISTEN_ADDR")
	if strings.TrimSpace(address) == "" {
		address = ":8080"
	}
	log.Printf("sms admin listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
