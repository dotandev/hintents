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

package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors are defined
	assert.NotNil(t, ErrTransactionNotFound)
	assert.NotNil(t, ErrRPCConnectionFailed)
	assert.NotNil(t, ErrSimulatorNotFound)
	assert.NotNil(t, ErrSimulationFailed)
	assert.NotNil(t, ErrInvalidNetwork)
	assert.NotNil(t, ErrMarshalFailed)
	assert.NotNil(t, ErrUnmarshalFailed)
	assert.NotNil(t, ErrSimulationLogicError)
}

func TestErrorWrapping(t *testing.T) {
	baseErr := fmt.Errorf("base error")

	// Test WrapTransactionNotFound
	wrappedErr := WrapTransactionNotFound(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrTransactionNotFound))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapRPCConnectionFailed
	wrappedErr = WrapRPCConnectionFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrRPCConnectionFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapSimulatorNotFound
	wrappedErr = WrapSimulatorNotFound("test message")
	assert.True(t, errors.Is(wrappedErr, ErrSimulatorNotFound))
	assert.Contains(t, wrappedErr.Error(), "test message")

	// Test WrapSimulationFailed
	wrappedErr = WrapSimulationFailed(baseErr, "stderr output")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "stderr output")

	// Test WrapInvalidNetwork
	wrappedErr = WrapInvalidNetwork("invalid")
	assert.True(t, errors.Is(wrappedErr, ErrInvalidNetwork))
	assert.Contains(t, wrappedErr.Error(), "invalid")
	assert.Contains(t, wrappedErr.Error(), "testnet, mainnet, futurenet")

	// Test WrapMarshalFailed
	wrappedErr = WrapMarshalFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrMarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test WrapUnmarshalFailed
	wrappedErr = WrapUnmarshalFailed(baseErr, "output")
	assert.True(t, errors.Is(wrappedErr, ErrUnmarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "output")

	// Test WrapSimulationLogicError
	wrappedErr = WrapSimulationLogicError("logic error")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationLogicError))
	assert.Contains(t, wrappedErr.Error(), "logic error")
}

func TestErrorComparison(t *testing.T) {
	// Test that different error types are distinguishable
	err1 := WrapTransactionNotFound(fmt.Errorf("test"))
	err2 := WrapRPCConnectionFailed(fmt.Errorf("test"))

	assert.True(t, errors.Is(err1, ErrTransactionNotFound))
	assert.False(t, errors.Is(err1, ErrRPCConnectionFailed))

	assert.True(t, errors.Is(err2, ErrRPCConnectionFailed))
	assert.False(t, errors.Is(err2, ErrTransactionNotFound))
}
