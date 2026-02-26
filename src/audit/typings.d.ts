// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

// Minimal module declarations to keep TS builds working in environments without types packages.

declare module 'fast-json-stable-stringify' {
  const stringify: (value: any) => string;
  export default stringify;
}
