package httpserver

import (
	"crypto/x509"
	"testing"
)

func TestLoadOrCreateCert_NotACA(t *testing.T) {
	cert, err := loadOrCreateCert(t.TempDir())
	if err != nil {
		t.Fatalf("loadOrCreateCert: %v", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	if leaf.IsCA {
		t.Error("self-signed TLS server leaf must not be marked IsCA")
	}
	if leaf.ExtKeyUsage == nil {
		t.Error("expected ServerAuth ext key usage")
	}
}
