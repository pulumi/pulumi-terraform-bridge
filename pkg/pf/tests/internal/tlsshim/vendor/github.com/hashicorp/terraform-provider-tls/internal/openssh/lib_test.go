package openssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestOpenSSHFormat_MarshalAndUnmarshal_RSA(t *testing.T) {
    t.Parallel()
	// Given an RSA private key
	rsaOrig, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Errorf("Failed to generate RSA private key: %v", err)
	}

	// Marshal it to OpenSSH PEM format
	pemOpenSSHPrvKey, err := MarshalPrivateKey(rsaOrig, "")
	if err != nil {
		t.Errorf("Failed to marshal RSA private key to OpenSSH PEM: %v", err)
	}
	pemOpenSSHPrvKeyBytes := pem.EncodeToMemory(pemOpenSSHPrvKey)

	// Parse it back into an RSA private key
	rawPrivateKey, _ := ssh.ParseRawPrivateKey(pemOpenSSHPrvKeyBytes)
	rsaParsed, ok := rawPrivateKey.(*rsa.PrivateKey)
	if !ok {
		t.Errorf("Failed to type assert RSA private key: %v", rawPrivateKey)
	}

	// Confirm RSA is valid
	err = rsaParsed.Validate()
	if err != nil {
		t.Errorf("Parsed RSA private key is not valid: %v", err)
	}
	// Confirm it matches the original key by comparing the public ones
	if !rsaParsed.Equal(rsaOrig) {
		t.Errorf("Parsed RSA private key doesn't match the original")
	}
}

func TestOpenSSHFormat_MarshalAndUnmarshal_ECDSA(t *testing.T) {
    t.Parallel()
	// Given an ECDSA private key
	ecdsaOrig, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Errorf("Failed to generate ECDSA private key: %v", err)
	}

	// Marshal it to OpenSSH PEM format
	pemOpenSSHPrvKey, err := MarshalPrivateKey(ecdsaOrig, "")
	if err != nil {
		t.Errorf("Failed to marshal ECDSA private key to OpenSSH PEM: %v", err)
	}
	pemOpenSSHPrvKeyBytes := pem.EncodeToMemory(pemOpenSSHPrvKey)

	// Parse it back into an ECDSA private key
	rawPrivateKey, _ := ssh.ParseRawPrivateKey(pemOpenSSHPrvKeyBytes)
	ecdsaParsed, ok := rawPrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Errorf("Failed to type assert ECDSA private key: %v", rawPrivateKey)
	}

	// Confirm it matches the original key by comparing the public ones
	if !ecdsaParsed.Equal(ecdsaOrig) {
		t.Errorf("Parsed ECDSA private key doesn't match the original")
	}
}

func TestOpenSSHFormat_MarshalAndUnmarshal_ED25519(t *testing.T) {
    t.Parallel()
	// Given an ED25519 private key
	_, ed25519Orig, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Errorf("Failed to generate ED25519 private key: %v", err)
	}

	// Marshal it to OpenSSH PEM format
	pemOpenSSHPrvKey, err := MarshalPrivateKey(ed25519Orig, "")
	if err != nil {
		t.Errorf("Failed to marshal ED25519 private key to OpenSSH PEM: %v", err)
	}
	pemOpenSSHPrvKeyBytes := pem.EncodeToMemory(pemOpenSSHPrvKey)

	// Parse it back into an ED25519 private key
	rawPrivateKey, _ := ssh.ParseRawPrivateKey(pemOpenSSHPrvKeyBytes)
	ed25519Parsed, ok := rawPrivateKey.(*ed25519.PrivateKey)
	if !ok {
		t.Errorf("Failed to type assert ED25519 private key: %v", rawPrivateKey)
	}

	// Confirm it matches the original key by comparing the public ones
	if !ed25519Parsed.Equal(ed25519Orig) {
		t.Errorf("Parsed ED25519 private key doesn't match the original")
	}
}
