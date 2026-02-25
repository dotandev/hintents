# YubiKey PIV (YKCS11) audit:sign

This guide documents how to use a YubiKey in PIV mode with the PKCS#11 signer for `audit:sign`.

## Requirements

- YubiKey with PIV enabled
- YKCS11 module (typically `libykcs11.so` from Yubico, often in `/usr/local/lib` or `/usr/lib`)
- A PIV key and certificate in the slot you plan to use

## Environment variables

Required:

- `ERST_PKCS11_MODULE`
- `ERST_PKCS11_PIN`
- `ERST_PKCS11_PUBLIC_KEY_PEM`
- One of `ERST_PKCS11_KEY_LABEL`, `ERST_PKCS11_KEY_ID`, or `ERST_PKCS11_PIV_SLOT`

Optional:

- `ERST_PKCS11_TOKEN_LABEL` to select the correct YubiKey if multiple tokens are present
- `ERST_PKCS11_SLOT` to select a slot by index instead of token label

## PIV slot to key ID mapping (YKCS11)

YKCS11 exposes PIV keys with PKCS#11 CKA_ID values that map to PIV slots. Use `ERST_PKCS11_PIV_SLOT` with one of the following values and the signer will derive the correct key ID:

- `9a` -> key ID `01`
- `9c` -> key ID `02`
- `9d` -> key ID `03`
- `9e` -> key ID `04`
- `82` to `95` -> key ID `05` to `18`
- `f9` -> key ID `19`

## Example

```bash
export ERST_PKCS11_MODULE=/usr/local/lib/libykcs11.so
export ERST_PKCS11_PIN=123456
export ERST_PKCS11_TOKEN_LABEL="YubiKey PIV"
export ERST_PKCS11_PIV_SLOT=9c
export ERST_PKCS11_PUBLIC_KEY_PEM="$(cat ./yubikey-piv-spki.pem)"

node dist/index.js audit:sign \
  --hsm-provider pkcs11 \
  --payload '{"input":{},"state":{},"events":[],"timestamp":"2026-01-01T00:00:00.000Z"}'
```

## Notes

- PIV slot `9c` is typically used for Digital Signature keys.
- If you already know the PKCS#11 key label exposed by YKCS11, you can use `ERST_PKCS11_KEY_LABEL` instead of `ERST_PKCS11_PIV_SLOT`.
- The public key should be provided as SPKI PEM. If you only have a PIV certificate, extract the SPKI public key with `openssl` and set `ERST_PKCS11_PUBLIC_KEY_PEM`.
