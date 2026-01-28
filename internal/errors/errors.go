package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for comparison with errors.Is
var (
	ErrTransactionNotFound   = errors.New("transaction not found")
	ErrRPCConnectionFailed   = errors.New("RPC connection failed")
	ErrSimulatorNotFound     = errors.New("simulator binary not found")
	ErrSimulationFailed      = errors.New("simulation execution failed")
	ErrInvalidNetwork        = errors.New("invalid network")
	ErrMarshalFailed         = errors.New("failed to marshal request")
	ErrUnmarshalFailed       = errors.New("failed to unmarshal response")
	ErrSimulationLogicError  = errors.New("simulation logic error")
)

// Wrap functions for consistent error wrapping
func WrapTransactionNotFound(err error) error {
	return fmt.Errorf("%w: %v", ErrTransactionNotFound, err)
}

func WrapRPCConnectionFailed(err error) error {
	return fmt.Errorf("%w: %v", ErrRPCConnectionFailed, err)
}

func WrapSimulatorNotFound(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulatorNotFound, msg)
}

func WrapSimulationFailed(err error, stderr string) error {
	return fmt.Errorf("%w: %v, stderr: %s", ErrSimulationFailed, err, stderr)
}

func WrapInvalidNetwork(network string) error {
	return fmt.Errorf("%w: %s. Must be one of: testnet, mainnet, futurenet", ErrInvalidNetwork, network)
}

func WrapMarshalFailed(err error) error {
	return fmt.Errorf("%w: %v", ErrMarshalFailed, err)
}

func WrapUnmarshalFailed(err error, output string) error {
	return fmt.Errorf("%w: %v, output: %s", ErrUnmarshalFailed, err, output)
}

func WrapSimulationLogicError(msg string) error {
	return fmt.Errorf("%w: %s", ErrSimulationLogicError, msg)
}
