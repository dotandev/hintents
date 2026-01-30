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
)

// Sentinel errors for comparison with errors.Is
var (
	// Core simulation and RPC errors
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrRPCConnectionFailed  = errors.New("RPC connection failed")
	ErrSimulatorNotFound    = errors.New("simulator binary not found")
	ErrSimulationFailed     = errors.New("simulation execution failed")
	ErrInvalidNetwork       = errors.New("invalid network")
	ErrMarshalFailed        = errors.New("failed to marshal request")
	ErrUnmarshalFailed      = errors.New("failed to unmarshal response")
	ErrSimulationLogicError = errors.New("simulation logic error")

	// File I/O errors
	ErrFileReadFailed         = errors.New("failed to read file")
	ErrFileCreationFailed     = errors.New("failed to create file")
	ErrFileWriteFailed        = errors.New("failed to write file")
	ErrDirectoryCreationFailed = errors.New("failed to create directory")

	// Database errors
	ErrDatabaseOpenFailed         = errors.New("failed to open database")
	ErrDatabaseInitializationFailed = errors.New("failed to initialize database")
	ErrSchemaMigrationFailed      = errors.New("failed to migrate schema")

	// Session errors
	ErrSessionStoreOpenFailed = errors.New("failed to open session store")
	ErrSessionSaveFailed      = errors.New("failed to save session")
	ErrSessionLoadFailed      = errors.New("failed to load session")
	ErrSessionDeleteFailed    = errors.New("failed to delete session")
	ErrSessionListFailed      = errors.New("failed to list sessions")
	ErrSessionCleanupFailed   = errors.New("failed to cleanup sessions")
	ErrNoActiveSession        = errors.New("no active session")

	// RPC/Network errors
	ErrRequestCreationFailed      = errors.New("failed to create request")
	ErrResponseReadFailed         = errors.New("failed to read response")
	ErrRPCError                   = errors.New("RPC error")
	ErrTransactionFetchFailed     = errors.New("failed to fetch transaction")
	ErrLedgerEntriesFetchFailed   = errors.New("failed to fetch ledger entries")
	ErrLedgerKeyExtractionFailed  = errors.New("failed to extract ledger keys")

	// Snapshot/Config errors
	ErrSnapshotReadFailed  = errors.New("failed to read snapshot")
	ErrSnapshotParseFailed = errors.New("failed to parse snapshot")
	ErrSnapshotWriteFailed = errors.New("failed to write snapshot")
	ErrConfigReadFailed    = errors.New("failed to read config")
	ErrConfigWriteFailed   = errors.New("failed to write config")

	// Webhook errors
	ErrInvalidWebhookURL    = errors.New("invalid webhook URL")
	ErrWebhookRequestFailed = errors.New("failed to create webhook request")
	ErrWebhookSendFailed    = errors.New("failed to send webhook request")

	// Simulator/Execution errors
	ErrSimulatorInitializationFailed = errors.New("failed to initialize simulator")
	ErrContractIdentificationFailed  = errors.New("failed to identify contract")
	ErrCodeInjectionFailed           = errors.New("failed to inject code")

	// Template/Generation errors
	ErrTemplateParsingFailed   = errors.New("failed to parse template")
	ErrTemplateExecutionFailed = errors.New("failed to execute template")

	// Validation errors
	ErrInvalidPublicKey      = errors.New("invalid public key")
	ErrInvalidSignature      = errors.New("invalid signature")
	ErrInvalidExecutionState = errors.New("invalid execution state")
	ErrInvalidUserInput      = errors.New("invalid user input")

	// Other errors
	ErrInputReadFailed      = errors.New("failed to read input")
	ErrSearchFailed         = errors.New("search failed")
	ErrServerCreationFailed = errors.New("failed to create server")
	ErrGasModelReadFailed   = errors.New("failed to read gas model")
	ErrTraceFileReadFailed  = errors.New("failed to read trace file")
	ErrTraceFileParseFailed = errors.New("failed to parse trace file")
)

// Wrap functions for consistent error wrapping
func WrapTransactionNotFound(err error) error {
	return fmt.Errorf("%w: %w", ErrTransactionNotFound, err)
}

func WrapRPCConnectionFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrRPCConnectionFailed, err)
}

func WrapSimulatorNotFound(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulatorNotFound, msg)
}

func WrapSimulationFailed(err error, stderr string) error {
	return fmt.Errorf("%w: %w, stderr: %s", ErrSimulationFailed, err, stderr)
}

func WrapInvalidNetwork(network string) error {
	return fmt.Errorf("%w: %s. Must be one of: testnet, mainnet, futurenet", ErrInvalidNetwork, network)
}

func WrapMarshalFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrMarshalFailed, err)
}

func WrapUnmarshalFailed(err error, output string) error {
	return fmt.Errorf("%w: %w, output: %s", ErrUnmarshalFailed, err, output)
}

func WrapSimulationLogicError(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulationLogicError, msg)
}

// File I/O wrapper functions
func WrapFileReadFailed(err error, filename string) error {
	return fmt.Errorf("%w: %s: %w", ErrFileReadFailed, filename, err)
}

func WrapFileCreationFailed(err error, filename string) error {
	return fmt.Errorf("%w: %s: %w", ErrFileCreationFailed, filename, err)
}

func WrapFileWriteFailed(err error, filename string) error {
	return fmt.Errorf("%w: %s: %w", ErrFileWriteFailed, filename, err)
}

func WrapDirectoryCreationFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrDirectoryCreationFailed, path, err)
}

// Database wrapper functions
func WrapDatabaseOpenFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrDatabaseOpenFailed, path, err)
}

func WrapDatabaseInitializationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrDatabaseInitializationFailed, err)
}

func WrapSchemaMigrationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSchemaMigrationFailed, err)
}

// Session wrapper functions
func WrapSessionStoreOpenFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSessionStoreOpenFailed, err)
}

func WrapSessionSaveFailed(err error, sessionID string) error {
	return fmt.Errorf("%w: %s: %w", ErrSessionSaveFailed, sessionID, err)
}

func WrapSessionLoadFailed(err error, sessionID string) error {
	return fmt.Errorf("%w: %s: %w", ErrSessionLoadFailed, sessionID, err)
}

func WrapSessionDeleteFailed(err error, sessionID string) error {
	return fmt.Errorf("%w: %s: %w", ErrSessionDeleteFailed, sessionID, err)
}

func WrapSessionListFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSessionListFailed, err)
}

func WrapSessionCleanupFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSessionCleanupFailed, err)
}

func WrapNoActiveSession(msg string) error {
	return fmt.Errorf("%w: %s", ErrNoActiveSession, msg)
}

// RPC/Network wrapper functions
func WrapRequestCreationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrRequestCreationFailed, err)
}

func WrapResponseReadFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrResponseReadFailed, err)
}

func WrapRPCError(message string, code int) error {
	return fmt.Errorf("%w: %s (code %d)", ErrRPCError, message, code)
}

func WrapTransactionFetchFailed(err error, txHash string) error {
	return fmt.Errorf("%w: %s: %w", ErrTransactionFetchFailed, txHash, err)
}

func WrapLedgerEntriesFetchFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrLedgerEntriesFetchFailed, err)
}

func WrapLedgerKeyExtractionFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrLedgerKeyExtractionFailed, err)
}

// Snapshot/Config wrapper functions
func WrapSnapshotReadFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrSnapshotReadFailed, path, err)
}

func WrapSnapshotParseFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrSnapshotParseFailed, path, err)
}

func WrapSnapshotWriteFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrSnapshotWriteFailed, path, err)
}

func WrapConfigReadFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrConfigReadFailed, path, err)
}

func WrapConfigWriteFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrConfigWriteFailed, path, err)
}

// Webhook wrapper functions
func WrapInvalidWebhookURL(err error, url string) error {
	return fmt.Errorf("%w: %s: %w", ErrInvalidWebhookURL, url, err)
}

func WrapWebhookRequestFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrWebhookRequestFailed, err)
}

func WrapWebhookSendFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrWebhookSendFailed, err)
}

// Simulator/Execution wrapper functions
func WrapSimulatorInitializationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSimulatorInitializationFailed, err)
}

func WrapContractIdentificationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrContractIdentificationFailed, err)
}

func WrapCodeInjectionFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrCodeInjectionFailed, err)
}

// Template/Generation wrapper functions
func WrapTemplateParsingFailed(err error, templateName string) error {
	return fmt.Errorf("%w: %s: %w", ErrTemplateParsingFailed, templateName, err)
}

func WrapTemplateExecutionFailed(err error, templateName string) error {
	return fmt.Errorf("%w: %s: %w", ErrTemplateExecutionFailed, templateName, err)
}

// Validation wrapper functions
func WrapInvalidPublicKey(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidPublicKey, err)
}

func WrapInvalidSignature(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidSignature, err)
}

func WrapInvalidExecutionState(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidExecutionState, msg)
}

func WrapInvalidUserInput(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidUserInput, msg)
}

// Other wrapper functions
func WrapInputReadFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrInputReadFailed, err)
}

func WrapSearchFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrSearchFailed, err)
}

func WrapServerCreationFailed(err error) error {
	return fmt.Errorf("%w: %w", ErrServerCreationFailed, err)
}

func WrapGasModelReadFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrGasModelReadFailed, path, err)
}

func WrapTraceFileReadFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrTraceFileReadFailed, path, err)
}

func WrapTraceFileParseFailed(err error, path string) error {
	return fmt.Errorf("%w: %s: %w", ErrTraceFileParseFailed, path, err)
}
