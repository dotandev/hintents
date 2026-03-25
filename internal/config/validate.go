// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"

	"github.com/dotandev/hintents/internal/errors"
)

const maxRequestTimeout = 300

type TimeoutValidator struct{}

func (TimeoutValidator) Validate(cfg *Config) error {
	if cfg.RequestTimeout <= 0 {
		return errors.WrapValidationError("request_timeout must be greater than 0")
	}
	if cfg.RequestTimeout > maxRequestTimeout {
		return errors.WrapValidationError(
			fmt.Sprintf("request_timeout must be at most %d seconds, got %d", maxRequestTimeout, cfg.RequestTimeout),
		)
	}
	return nil
}

type CrashReportingValidator struct{}

func (CrashReportingValidator) Validate(cfg *Config) error {
	if !cfg.CrashReporting {
		return nil
	}
	if cfg.CrashEndpoint == "" && cfg.CrashSentryDSN == "" {
		return errors.WrapValidationError(
			"crash_reporting is enabled but neither crash_endpoint nor crash_sentry_dsn is set",
		)
	}
	if cfg.CrashSentryDSN != "" && !strings.HasPrefix(cfg.CrashSentryDSN, "https://") {
		return errors.WrapValidationError(
			fmt.Sprintf("crash_sentry_dsn must use https scheme, got %q", cfg.CrashSentryDSN),
		)
	}
	return nil
}
