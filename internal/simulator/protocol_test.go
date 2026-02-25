// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"testing"
)

func TestLatestVersion(t *testing.T) {
	v := LatestVersion()
	if v != 22 {
		t.Errorf("expected latest version 22, got %d", v)
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		version uint32
		wantErr bool
	}{
		{"protocol 20", 20, false},
		{"protocol 21", 21, false},
		{"protocol 22", 22, false},
		{"unsupported", 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Get(tt.version)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Get(%d) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
			if !tt.wantErr && p.Version != tt.version {
				t.Errorf("expected version %d, got %d", tt.version, p.Version)
			}
		})
	}
}

func TestGetOrDefault(t *testing.T) {
	p := GetOrDefault(nil)
	if p.Version != LatestVersion() {
		t.Errorf("expected default version %d, got %d", LatestVersion(), p.Version)
	}

	v := uint32(20)
	p = GetOrDefault(&v)
	if p.Version != 20 {
		t.Errorf("expected version 20, got %d", p.Version)
	}
}

func TestFeature(t *testing.T) {
	tests := []struct {
		version uint32
		key     string
		wantErr bool
	}{
		{20, "max_contract_size", false},
		{21, "max_instruction_limit", false},
		{22, "optimized_storage", false},
		{22, "nonexistent", true},
		{99, "max_contract_size", true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			_, err := Feature(tt.version, tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Feature(%d, %q) error = %v, wantErr %v", tt.version, tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestFeatureOrDefault(t *testing.T) {
	val := FeatureOrDefault(22, "optimized_storage", false)
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	val = FeatureOrDefault(22, "nonexistent", "fallback")
	if val != "fallback" {
		t.Errorf("expected 'fallback', got %v", val)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		version uint32
		wantErr bool
	}{
		{20, false},
		{21, false},
		{22, false},
		{99, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := Validate(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate(%d) error = %v, wantErr %v", tt.version, err, tt.wantErr)
			}
		})
	}
}

func TestSupported(t *testing.T) {
	versions := Supported()
	if len(versions) < 3 {
		t.Errorf("expected at least 3 protocols, got %d", len(versions))
	}

	for i := 0; i < len(versions)-1; i++ {
		if versions[i] >= versions[i+1] {
			t.Errorf("expected sorted versions, got %v", versions)
		}
	}
}

func TestMergeFeatures(t *testing.T) {
	custom := map[string]interface{}{
		"custom_limit":      999999,
		"max_contract_size": 131072,
	}

	merged := MergeFeatures(22, custom)

	if merged["custom_limit"] != 999999 {
		t.Errorf("expected custom_limit 999999, got %v", merged["custom_limit"])
	}

	if merged["max_contract_size"] != 131072 {
		t.Errorf("expected overridden max_contract_size 131072, got %v", merged["max_contract_size"])
	}

	if merged["optimized_storage"] != true {
		t.Errorf("expected base feature optimized_storage true, got %v", merged["optimized_storage"])
	}
}

func TestResourceCalibration(t *testing.T) {
	val, err := Feature(22, "resource_calibration")
	if err != nil {
		t.Fatalf("failed to get resource_calibration: %v", err)
	}

	calib, ok := val.(*ResourceCalibration)
	if !ok {
		t.Fatalf("expected *ResourceCalibration, got %T", val)
	}

	if calib.SHA256Fixed != 3738 {
		t.Errorf("expected SHA256Fixed 3738, got %d", calib.SHA256Fixed)
	}
	if calib.Ed25519Fixed != 377524 {
		t.Errorf("expected Ed25519Fixed 377524, got %d", calib.Ed25519Fixed)
	}
}
