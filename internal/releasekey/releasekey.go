// Package releasekey loads the OpenAPS release public key and verifies
// detached signatures made with the matching private key. The release key
// is the single trust anchor for everything the box pulls from the network:
// the TLS proxy verifies the package index against it, and the OTA path
// verifies update tarballs against it.
//
// The key is an RSA-2048 public key in PKIX/PEM form (the shape of
// /etc/openaps/release.pub). Signatures are RSASSA-PKCS1v15 over SHA-256 —
// the format produced by `openssl dgst -sha256 -sign priv.pem`, so a feed
// can be signed with stock openssl in CI without a bespoke tool.
package releasekey

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// minBits is the smallest RSA modulus accepted. The production key is 2048;
// anything smaller is rejected so a downgraded or test-weak key cannot be
// plumbed in by accident.
const minBits = 2048

// Verifier holds a parsed release public key and checks detached signatures
// against it. It is immutable after construction and safe for concurrent use.
type Verifier struct {
	pub *rsa.PublicKey
}

// Load parses a PKIX/PEM RSA public key (the release.pub shape) and returns
// a Verifier. It rejects non-RSA keys and moduli below minBits.
func Load(pemBytes []byte) (*Verifier, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("releasekey: no PEM block found")
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("releasekey: unexpected PEM type %q (want PUBLIC KEY)", block.Type)
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("releasekey: parse PKIX: %w", err)
	}
	pub, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("releasekey: key is %T, want RSA", parsed)
	}
	if pub.N.BitLen() < minBits {
		return nil, fmt.Errorf("releasekey: RSA key is %d bits, want >= %d", pub.N.BitLen(), minBits)
	}
	return &Verifier{pub: pub}, nil
}

// LoadFile reads a release public key from path and parses it.
func LoadFile(path string) (*Verifier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("releasekey: read %s: %w", path, err)
	}
	return Load(data)
}

// Verify reports whether sig is a valid RSASSA-PKCS1v15-SHA256 signature
// over message under the release key. It returns nil on success and a
// non-nil error on any mismatch, so callers can fail closed with one check.
func (v *Verifier) Verify(message, sig []byte) error {
	sum := sha256.Sum256(message)
	if err := rsa.VerifyPKCS1v15(v.pub, crypto.SHA256, sum[:], sig); err != nil {
		return fmt.Errorf("releasekey: signature verification failed: %w", err)
	}
	return nil
}
