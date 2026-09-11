package smsadmin

import "fmt"

type Lifecycle string

const (
	LifecycleOnboarding Lifecycle = "onboarding"
	LifecycleActive     Lifecycle = "active"
	LifecycleSuspended  Lifecycle = "suspended"
)

type AssetKind string

const (
	AssetSignature AssetKind = "signature"
	AssetTemplate  AssetKind = "template"
)

type CreateCommand struct {
	TenantID    string
	Lifecycle   Lifecycle
	Admin       bool
	Kind        AssetKind
	OperationID string
}

func (c CreateCommand) Validate() error {
	if c.TenantID == "" || c.OperationID == "" {
		return fmt.Errorf("tenant_id and operation_id are required")
	}
	if !c.Admin {
		return fmt.Errorf("admin role required")
	}
	if c.Lifecycle != LifecycleActive {
		return fmt.Errorf("tenant must be active")
	}
	if c.Kind != AssetSignature && c.Kind != AssetTemplate {
		return fmt.Errorf("asset_kind must be signature or template")
	}
	return nil
}

func (c CreateCommand) Path() string {
	if c.Kind == AssetSignature {
		return "/v1/sms/signature/create"
	}
	return "/v1/sms/template/create"
}
