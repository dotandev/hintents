// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

declare const process: {
  env: Record<string, string | undefined>;
  stdout: { write: (chunk: string | Uint8Array) => void };
  exit: (code?: number) => never;
};
