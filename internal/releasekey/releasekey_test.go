package releasekey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"crypto"
)

// testKeyPEM generates an RSA key of the given size and returns its PKIX/PEM
// public encoding alongside the private key for signing in the test.
func testKeyPEM(t *testing.T, bits int) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal pub: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), priv
}

func sign(t *testing.T, priv *rsa.PrivateKey, msg []byte) []byte {
	t.Helper()
	sum := sha256.Sum256(msg)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return sig
}

func TestVerifyAcceptsValidSignature(t *testing.T) {
	pemBytes, priv := testKeyPEM(t, 2048)
	v, err := Load(pemBytes)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	msg := []byte("Package: openaps-inv-driver\nVersion: 1.0.0\n")
	if err := v.Verify(msg, sign(t, priv, msg)); err != nil {
		t.Fatalf("verify valid: %v", err)
	}
}

func TestVerifyRejectsTamperedMessage(t *testing.T) {
	pemBytes, priv := testKeyPEM(t, 2048)
	v, _ := Load(pemBytes)
	msg := []byte("Package: openaps-inv-driver\nVersion: 1.0.0\n")
	sig := sign(t, priv, msg)
	tampered := append([]byte{}, msg...)
	tampered[0] ^= 0x01
	if err := v.Verify(tampered, sig); err == nil {
		t.Fatal("expected verification failure on tampered message")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	pemBytes, _ := testKeyPEM(t, 2048)
	_, otherPriv := testKeyPEM(t, 2048)
	v, _ := Load(pemBytes)
	msg := []byte("hello")
	if err := v.Verify(msg, sign(t, otherPriv, msg)); err == nil {
		t.Fatal("expected verification failure with signature from a different key")
	}
}

func TestLoadRejectsUndersizedKey(t *testing.T) {
	pemBytes, _ := testKeyPEM(t, 1024)
	if _, err := Load(pemBytes); err == nil {
		t.Fatal("expected rejection of 1024-bit key")
	}
}

func TestLoadRejectsNonRSAAndGarbage(t *testing.T) {
	if _, err := Load([]byte("not pem")); err == nil {
		t.Fatal("expected error on non-PEM input")
	}
	// Valid PEM, wrong block type.
	wrong := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x30, 0x00}})
	if _, err := Load(wrong); err == nil {
		t.Fatal("expected error on non PUBLIC KEY block")
	}
}

func TestLoadFile(t *testing.T) {
	pemBytes, priv := testKeyPEM(t, 2048)
	dir := t.TempDir()
	path := filepath.Join(dir, "release.pub")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	v, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	msg := []byte("x")
	if err := v.Verify(msg, sign(t, priv, msg)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := LoadFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("expected error for missing file")
	}
}
