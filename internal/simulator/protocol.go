// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"fmt"
	"maps"
	"sort"

	"github.com/dotandev/hintents/internal/errors"
)

type Protocol struct {
	Version  uint32
	Name     string
	Features map[string]interface{}
}

const (
	keccak256FixedCalibration   uint64 = 3072
	keccak256PerByteCalibration uint64 = 74
)

var protocols = map[uint32]*Protocol{
	20: {
		Version: 20,
		Name:    "Soroban Protocol 20",
		Features: map[string]interface{}{
			"max_contract_size":      65536,
			"max_contract_data_size": 1024000,
			"max_instruction_limit":  100000000,
			"supported_opcodes":      []string{"invoke_contract", "create_contract"},
			"resource_calibration": &ResourceCalibration{
				SHA256Fixed:      3738,
				SHA256PerByte:    37,
				Keccak256Fixed:   keccak256FixedCalibration,
				Keccak256PerByte: keccak256PerByteCalibration,
				Ed25519Fixed:     377524,
			},
		},
	},
	21: {
		Version: 21,
		Name:    "Soroban Protocol 21",
		Features: map[string]interface{}{
			"max_contract_size":      65536,
			"max_contract_data_size": 2048000,
			"max_instruction_limit":  150000000,
			"supported_opcodes":      []string{"invoke_contract", "create_contract", "extend_contract"},
			"enhanced_metering":      true,
			"resource_calibration": &ResourceCalibration{
				SHA256Fixed:      3738,
				SHA256PerByte:    37,
				Keccak256Fixed:   keccak256FixedCalibration,
				Keccak256PerByte: keccak256PerByteCalibration,
				Ed25519Fixed:     377524,
			},
		},
	},
	22: protocol22,
	// 23-27: the simulator's Rust host now runs protocol 27
	// (simulator/Cargo.toml pins soroban-env-host to the 27.x line), and
	// Validate() rejected any --protocol-version above 22 even though the
	// host itself accepts these. Feature metadata here is carried forward
	// unchanged from protocol 22 rather than guessed: this project has no
	// verified per-protocol calibration/limit data for 23-27, and the three
	// pre-existing entries (20-22) already share identical
	// resource_calibration numbers, so this doesn't introduce a new kind of
	// inaccuracy — it keeps requests at these protocols from being rejected
	// outright while leaving the real numbers as a known gap to fill in.
	23: protocolWithVersion(protocol22, 23, "Soroban Protocol 23"),
	24: protocolWithVersion(protocol22, 24, "Soroban Protocol 24"),
	25: protocolWithVersion(protocol22, 25, "Soroban Protocol 25"),
	26: protocolWithVersion(protocol22, 26, "Soroban Protocol 26"),
	27: protocolWithVersion(protocol22, 27, "Soroban Protocol 27"),
}

var protocol22 = &Protocol{
	Version: 22,
	Name:    "Soroban Protocol 22",
	Features: map[string]interface{}{
		"max_contract_size":      131072,
		"max_contract_data_size": 4096000,
		"max_instruction_limit":  200000000,
		"supported_opcodes":      []string{"invoke_contract", "create_contract", "extend_contract", "upgrade_contract"},
		"enhanced_metering":      true,
		"optimized_storage":      true,
		"resource_calibration": &ResourceCalibration{
			SHA256Fixed:      3738,
			SHA256PerByte:    37,
			Keccak256Fixed:   keccak256FixedCalibration,
			Keccak256PerByte: keccak256PerByteCalibration,
			Ed25519Fixed:     377524,
		},
	},
}

// protocolWithVersion clones the Features of an existing, verified protocol
// entry under a new version number and name. Used to extend protocol
// coverage to versions this project has not yet sourced real per-protocol
// feature/calibration data for (see the comment above the 23-27 entries).
func protocolWithVersion(base *Protocol, version uint32, name string) *Protocol {
	features := make(map[string]interface{}, len(base.Features))
	maps.Copy(features, base.Features)
	return &Protocol{
		Version:  version,
		Name:     name,
		Features: features,
	}
}

var defaultVersion uint32 = 27

func LatestVersion() uint32 {
	return defaultVersion
}

func Get(version uint32) (*Protocol, error) {
	if p, exists := protocols[version]; exists {
		return p, nil
	}
	return nil, errors.WrapProtocolUnsupported(version)
}

func GetOrDefault(version *uint32) *Protocol {
	if version == nil || *version == 0 {
		return protocols[defaultVersion]
	}
	p, _ := Get(*version)
	if p == nil {
		return protocols[defaultVersion]
	}
	return p
}

func Feature(version uint32, key string) (interface{}, error) {
	p, err := Get(version)
	if err != nil {
		return nil, err
	}
	val, exists := p.Features[key]
	if !exists {
		return nil, errors.WrapSimulationLogicError(fmt.Sprintf("feature %q not found in protocol %d", key, version))
	}
	return val, nil
}

func FeatureOrDefault(version uint32, key string, defVal interface{}) interface{} {
	val, err := Feature(version, key)
	if err != nil {
		return defVal
	}
	return val
}

func Validate(version uint32) error {
	if _, ok := protocols[version]; !ok {
		return errors.WrapProtocolUnsupported(version)
	}
	return nil
}

func Supported() []uint32 {
	versions := make([]uint32, 0, len(protocols))
	for v := range protocols {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})
	return versions
}

func MergeFeatures(version uint32, custom map[string]interface{}) map[string]interface{} {
	p, _ := Get(version)
	if p == nil {
		return custom
	}
	result := maps.Clone(p.Features)
	for k, v := range custom {
		result[k] = v
	}
	return result
}
