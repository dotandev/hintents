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
	// Test that all sentinel errors are defined
	sentinelErrors := []error{
		// Core simulation and RPC errors
		ErrTransactionNotFound,
		ErrRPCConnectionFailed,
		ErrSimulatorNotFound,
		ErrSimulationFailed,
		ErrInvalidNetwork,
		ErrMarshalFailed,
		ErrUnmarshalFailed,
		ErrSimulationLogicError,

		// File I/O errors
		ErrFileReadFailed,
		ErrFileCreationFailed,
		ErrFileWriteFailed,
		ErrDirectoryCreationFailed,

		// Database errors
		ErrDatabaseOpenFailed,
		ErrDatabaseInitializationFailed,
		ErrSchemaMigrationFailed,

		// Session errors
		ErrSessionStoreOpenFailed,
		ErrSessionSaveFailed,
		ErrSessionLoadFailed,
		ErrSessionDeleteFailed,
		ErrSessionListFailed,
		ErrSessionCleanupFailed,
		ErrNoActiveSession,

		// RPC/Network errors
		ErrRequestCreationFailed,
		ErrResponseReadFailed,
		ErrRPCError,
		ErrTransactionFetchFailed,
		ErrLedgerEntriesFetchFailed,
		ErrLedgerKeyExtractionFailed,

		// Snapshot/Config errors
		ErrSnapshotReadFailed,
		ErrSnapshotParseFailed,
		ErrSnapshotWriteFailed,
		ErrConfigReadFailed,
		ErrConfigWriteFailed,

		// Webhook errors
		ErrInvalidWebhookURL,
		ErrWebhookRequestFailed,
		ErrWebhookSendFailed,

		// Simulator/Execution errors
		ErrSimulatorInitializationFailed,
		ErrContractIdentificationFailed,
		ErrCodeInjectionFailed,

		// Template/Generation errors
		ErrTemplateParsingFailed,
		ErrTemplateExecutionFailed,

		// Validation errors
		ErrInvalidPublicKey,
		ErrInvalidSignature,
		ErrInvalidExecutionState,
		ErrInvalidUserInput,

		// Other errors
		ErrInputReadFailed,
		ErrSearchFailed,
		ErrServerCreationFailed,
		ErrGasModelReadFailed,
		ErrTraceFileReadFailed,
		ErrTraceFileParseFailed,
	}

	for _, err := range sentinelErrors {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}

func TestErrorWrapping(t *testing.T) {
	baseErr := fmt.Errorf("base error")

	// Test core error wrapping
	wrappedErr := WrapTransactionNotFound(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrTransactionNotFound))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapRPCConnectionFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrRPCConnectionFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapSimulatorNotFound("test message")
	assert.True(t, errors.Is(wrappedErr, ErrSimulatorNotFound))
	assert.Contains(t, wrappedErr.Error(), "test message")

	wrappedErr = WrapSimulationFailed(baseErr, "stderr output")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "stderr output")

	wrappedErr = WrapInvalidNetwork("invalid")
	assert.True(t, errors.Is(wrappedErr, ErrInvalidNetwork))
	assert.Contains(t, wrappedErr.Error(), "invalid")
	assert.Contains(t, wrappedErr.Error(), "testnet, mainnet, futurenet")

	wrappedErr = WrapMarshalFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrMarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapUnmarshalFailed(baseErr, "output")
	assert.True(t, errors.Is(wrappedErr, ErrUnmarshalFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "output")

	wrappedErr = WrapSimulationLogicError("logic error")
	assert.True(t, errors.Is(wrappedErr, ErrSimulationLogicError))
	assert.Contains(t, wrappedErr.Error(), "logic error")

	// Test file I/O error wrapping
	wrappedErr = WrapFileReadFailed(baseErr, "test.txt")
	assert.True(t, errors.Is(wrappedErr, ErrFileReadFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "test.txt")

	wrappedErr = WrapFileCreationFailed(baseErr, "output.txt")
	assert.True(t, errors.Is(wrappedErr, ErrFileCreationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "output.txt")

	wrappedErr = WrapDirectoryCreationFailed(baseErr, "/tmp/test")
	assert.True(t, errors.Is(wrappedErr, ErrDirectoryCreationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "/tmp/test")

	// Test database error wrapping
	wrappedErr = WrapDatabaseOpenFailed(baseErr, "test.db")
	assert.True(t, errors.Is(wrappedErr, ErrDatabaseOpenFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "test.db")

	wrappedErr = WrapSchemaMigrationFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrSchemaMigrationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test session error wrapping
	wrappedErr = WrapSessionStoreOpenFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrSessionStoreOpenFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapSessionSaveFailed(baseErr, "session123")
	assert.True(t, errors.Is(wrappedErr, ErrSessionSaveFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "session123")

	wrappedErr = WrapNoActiveSession("run debug first")
	assert.True(t, errors.Is(wrappedErr, ErrNoActiveSession))
	assert.Contains(t, wrappedErr.Error(), "run debug first")

	// Test RPC error wrapping
	wrappedErr = WrapRequestCreationFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrRequestCreationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapRPCError("timeout", 500)
	assert.True(t, errors.Is(wrappedErr, ErrRPCError))
	assert.Contains(t, wrappedErr.Error(), "timeout")
	assert.Contains(t, wrappedErr.Error(), "500")

	wrappedErr = WrapTransactionFetchFailed(baseErr, "abc123")
	assert.True(t, errors.Is(wrappedErr, ErrTransactionFetchFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "abc123")

	// Test snapshot error wrapping
	wrappedErr = WrapSnapshotReadFailed(baseErr, "snapshot.json")
	assert.True(t, errors.Is(wrappedErr, ErrSnapshotReadFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "snapshot.json")

	// Test webhook error wrapping
	wrappedErr = WrapInvalidWebhookURL(baseErr, "invalid-url")
	assert.True(t, errors.Is(wrappedErr, ErrInvalidWebhookURL))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "invalid-url")

	// Test simulator error wrapping
	wrappedErr = WrapSimulatorInitializationFailed(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrSimulatorInitializationFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	// Test template error wrapping
	wrappedErr = WrapTemplateParsingFailed(baseErr, "go_test")
	assert.True(t, errors.Is(wrappedErr, ErrTemplateParsingFailed))
	assert.True(t, errors.Is(wrappedErr, baseErr))
	assert.Contains(t, wrappedErr.Error(), "go_test")

	// Test validation error wrapping
	wrappedErr = WrapInvalidPublicKey(baseErr)
	assert.True(t, errors.Is(wrappedErr, ErrInvalidPublicKey))
	assert.True(t, errors.Is(wrappedErr, baseErr))

	wrappedErr = WrapInvalidUserInput("invalid selection")
	assert.True(t, errors.Is(wrappedErr, ErrInvalidUserInput))
	assert.Contains(t, wrappedErr.Error(), "invalid selection")
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
