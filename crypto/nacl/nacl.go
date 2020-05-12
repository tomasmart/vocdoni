// Package nacl implements encryption and decryption using anonymous sealed
// boxes, depending on golang.org/x/crypto/nacl/box.
package nacl

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
)

const KeyLength = 32

// DecodeKey decodes a public or private key from a hexadecimal string.
func DecodeKey(hexkey string) (*[KeyLength]byte, error) {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, err
	}
	if len(b) != KeyLength {
		return nil, fmt.Errorf("key length must be %d, not %d", KeyLength, len(b))
	}
	key := new([KeyLength]byte)
	copy(key[:], b)
	return key, nil
}

// KeyPair holds pair of public and private keys.
type KeyPair struct {
	Public, Private [KeyLength]byte
}

// Generate creates a new random KeyPair. If randReader is nil,
// crypto/rand.Reader is used.
func Generate(randReader io.Reader) (*KeyPair, error) {
	if randReader == nil {
		randReader = cryptorand.Reader
	}
	pub, priv, err := box.GenerateKey(randReader)
	if err != nil {
		return nil, err
	}
	return &KeyPair{Public: *pub, Private: *priv}, nil
}

// FromHex creates a KeyPair from the provided hexadecimal private key.
func FromHex(privHex string) (*KeyPair, error) {
	priv, err := DecodeKey(privHex)
	if err != nil {
		return nil, err
	}
	kp := &KeyPair{Private: *priv}

	pub, err := curve25519.X25519(kp.Private[:], curve25519.Basepoint)
	if err != nil {
		return kp, err
	}
	copy(kp.Public[:], pub)

	return kp, nil
}

// Encrypt is a standalone version of KeyPair.Encrypt, since the recipient's
// private key isn't needed to encrypt.
func Encrypt(message []byte, public *[KeyLength]byte) ([]byte, error) {
	return box.SealAnonymous(nil, message, public, cryptorand.Reader)
}

func (k *KeyPair) Hex() (string, string) {
	return hex.EncodeToString(k.Public[:]), hex.EncodeToString(k.Private[:])
}

func (k *KeyPair) Encrypt(message []byte) ([]byte, error) {
	return Encrypt(message, &k.Public)
}

func (k *KeyPair) Decrypt(cipher []byte) ([]byte, error) {
	message, ok := box.OpenAnonymous(nil, cipher, &k.Public, &k.Private)
	if !ok {
		return nil, fmt.Errorf("could not open box")
	}
	return message, nil
}