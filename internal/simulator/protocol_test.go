// Copyright (c) 2026 dotandev
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		version  uint32
		key      string
		wantErr  bool
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
		"custom_limit": 999999,
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
