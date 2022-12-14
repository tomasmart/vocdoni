package ethereum

import (
	"bytes"
	"encoding/hex"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestSignature(t *testing.T) {
	t.Parallel()

	s := NewSignKeys()
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub:%s priv:%s", pub, priv)
	message := []byte("hello")
	t.Logf("Message to sign: %s", message)
	msgSign, err := s.SignEthereum(message)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature is %s", msgSign)

	s2 := NewSignKeys()
	err = s2.AddHexKey(priv)
	qt.Assert(t, err, qt.IsNil)
	pub, priv = s2.HexString()
	t.Logf("Imported pub:%s priv:%s", pub, priv)
	s2.AddAuthKey(s.Address())
	v, _, err := s2.VerifySender(message, msgSign)
	qt.Assert(t, err, qt.IsNil)
	if !v {
		t.Fatal("Verification failed!")
	}
	t.Logf("Testing verification... %t", v)

	t.Log("Testing compatibility with standard Ethereum signing libraries")
	hardcodedPriv := "fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19"
	hardcodedSignature, err := hex.DecodeString("a0d0ebc374d2a4d6357eaca3da2f5f3ff547c3560008206bc234f9032a866ace6279ffb4093fb39c8bbc39021f6a5c36ef0e813c8c94f325a53f4f395a5c82de01")
	qt.Assert(t, err, qt.IsNil)
	s3 := NewSignKeys()
	err = s3.AddHexKey(hardcodedPriv)
	qt.Assert(t, err, qt.IsNil)
	_, priv = s3.HexString()
	if priv != hardcodedPriv {
		t.Fatalf("PrivKey from %s not match the hardcoded one\nGot %s\nMust have %s", hardcodedPriv, priv, hardcodedPriv[2:])
	}
	signature, err := s3.SignEthereum(message)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature: %s", signature)
	if !bytes.Equal(signature, hardcodedSignature) {
		t.Fatalf("Hardcoded signature %s do not match", hardcodedSignature)
	}
}

func TestAddr(t *testing.T) {
	t.Parallel()

	s := NewSignKeys()
	if err := s.Generate(); err != nil {
		t.Fatal(err)
	}
	pub, priv := s.HexString()
	t.Logf("Generated pub: %s \npriv: %s", pub, priv)
	addr1 := s.AddressString()
	addr2, err := AddrFromPublicKey(s.PublicKey())
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Recovered address from pubKey %s", addr2)
	if addr1 != addr2.String() {
		t.Fatalf("Calculated address from pubKey do not match: %s != %s", addr1, addr2)
	}
	msg := []byte("hello vocdoni")
	signature, err := s.SignEthereum(msg)
	qt.Assert(t, err, qt.IsNil)
	t.Logf("Signature created: %s", signature)
	addr3, err := AddrFromSignature(msg, signature)
	qt.Assert(t, err, qt.IsNil)

	if addr3 != addr2 {
		t.Fatalf("Extracted signature address do not match: %s != %s", addr2, addr3)
	}

	s.AddAuthKey(addr3)
	v, _, err := s.VerifySender(msg, signature)
	qt.Assert(t, err, qt.IsNil)
	if !v {
		t.Fatal("Cannot verify sender")
	}

	v, _, err = s.VerifySender(msg, signature)
	qt.Assert(t, err, qt.IsNil)
	if !v {
		t.Fatal("Cannot verify signature")
	}

	msg = []byte("bye-bye vocdoni")
	signature2, err := s.SignEthereum(msg)
	qt.Assert(t, err, qt.IsNil)
	addr4, err := AddrFromSignature(msg, signature2)
	qt.Assert(t, err, qt.IsNil)
	if addr4 != addr3 {
		t.Fatal("extracted address from second message do not match")
	}
	t.Logf("%s == %s", addr3, addr4)
}
