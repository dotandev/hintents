// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package protocolreg

import (
	"crypto/sha256"
	"fmt"

	"github.com/miekg/pkcs11"
)

// DeriveKeyFromHSM initializes the PKCS#11 module, logs in, and derives a 32-byte encryption key.
func DeriveKeyFromHSM(libPath, pin, keyLabel string) ([]byte, error) {
	p := pkcs11.New(libPath)
	if err := p.Initialize(); err != nil {
		return nil, fmt.Errorf("hsm init failed: %w", err)
	}
	defer p.Destroy()
	defer p.Finalize()

	slots, err := p.GetSlotList(true)
	if err != nil || len(slots) == 0 {
		return nil, fmt.Errorf("no slots available")
	}

	session, err := p.OpenSession(slots[0], pkcs11.CKF_SERIAL_SESSION|pkcs11.CKF_RW_SESSION)
	if err != nil {
		return nil, fmt.Errorf("open session failed: %w", err)
	}
	defer p.CloseSession(session)

	if err := p.Login(session, pkcs11.CKU_USER, pin); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	defer p.Logout(session)

	template := []*pkcs11.Attribute{
		pkcs11.NewAttribute(pkcs11.CKA_LABEL, keyLabel),
		pkcs11.NewAttribute(pkcs11.CKA_CLASS, pkcs11.CKO_PRIVATE_KEY),
	}
	if err := p.FindObjectsInit(session, template); err != nil {
		return nil, err
	}
	objs, _, err := p.FindObjects(session, 1)
	if err != nil {
		return nil, err
	}
	if err := p.FindObjectsFinal(session); err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, fmt.Errorf("key not found")
	}

	mech := []*pkcs11.Mechanism{pkcs11.NewMechanism(pkcs11.CKM_RSA_PKCS, nil)}
	if err := p.SignInit(session, mech, objs[0]); err != nil {
		return nil, err
	}

	seed := []byte("ErstSessionDBKey-Derivation-Seed")
	sig, err := p.Sign(session, seed)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(sig)
	return hash[:], nil
}
