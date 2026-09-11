package smsadmin

import "testing"

func TestCreateCommandPolicy(t *testing.T) {
	tests := []struct {
		name    string
		command CreateCommand
		wantErr bool
	}{
		{"active admin creates template", CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleActive, Admin: true, Kind: AssetTemplate, OperationID: "op-1"}, false},
		{"onboarding tenant waits for activation", CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleOnboarding, Admin: true, Kind: AssetSignature, OperationID: "op-2"}, true},
		{"suspended tenant cannot create", CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleSuspended, Admin: true, Kind: AssetTemplate, OperationID: "op-3"}, true},
		{"member cannot administer assets", CreateCommand{TenantID: "tenant-a", Lifecycle: LifecycleActive, Admin: false, Kind: AssetSignature, OperationID: "op-4"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.command.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
