// Copyright (c) Hintents Authors.
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unused-vars */

import { KmsSigner } from "../src/audit/signing/kmsSigner";
import { SoftwareEd25519Signer } from "../src/audit/signing/softwareSigner";
import { Pkcs11Ed25519Signer } from "../src/audit/signing/pkcs11Signer";

jest.mock("../src/audit/signing/kmsSigner");
jest.mock("../src/audit/signing/softwareSigner");
jest.mock("../src/audit/signing/pkcs11Signer");

describe("AuditSigner Factory", () => {
  const mockPrivateKey = process.env.TEST_PRIVATE_KEY_PEM || "";
  const mockPublicKey = process.env.TEST_PUBLIC_KEY_PEM || "";

  beforeEach(() => {
    process.env.ERST_KMS_KEY_ID = "arn:aws:kms:us-east-1:123456789012:key/test";
    process.env.ERST_KMS_PUBLIC_KEY_PEM = mockPublicKey;
    jest.clearAllMocks();
  });

  afterEach(() => {
    delete process.env.ERST_KMS_KEY_ID;
    delete process.env.ERST_KMS_PUBLIC_KEY_PEM;
    delete process.env.ERST_PKCS11_MODULE;
  });

  test("creates KMS signer when provider is kms", () => {
    createAuditSigner({ hsmProvider: "kms" });
    expect(KmsSigner).toHaveBeenCalled();
  });

  test("creates KMS signer with case-insensitive provider", () => {
    createAuditSigner({ hsmProvider: "KMS" });
    expect(KmsSigner).toHaveBeenCalled();
  });

  test("creates software signer when provider is software", () => {
    createAuditSigner({
      hsmProvider: "software",
      softwarePrivateKeyPem: mockPrivateKey,
    });
    expect(SoftwareEd25519Signer).toHaveBeenCalledWith(mockPrivateKey);
  });

  test("creates software signer by default", () => {
    createAuditSigner({ softwarePrivateKeyPem: mockPrivateKey });
    expect(SoftwareEd25519Signer).toHaveBeenCalledWith(mockPrivateKey);
  });

  test("creates PKCS#11 signer when provider is pkcs11", () => {
    process.env.ERST_PKCS11_MODULE = "/usr/lib/softhsm/libsofthsm2.so";
    createAuditSigner({ hsmProvider: "pkcs11" });
    expect(Pkcs11Ed25519Signer).toHaveBeenCalled();
  });

  test("throws when software provider without private key", () => {
    expect(() => createAuditSigner({ hsmProvider: "software" })).toThrow(
      "software signing selected but no private key was provided",
    );
  });
});
