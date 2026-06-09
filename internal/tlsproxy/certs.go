package tlsproxy

import (
	"crypto/x509"
	_ "embed"
	"fmt"
	"os"
)

// caBundle is the Mozilla CA root set, vendored from
// https://curl.se/ca/cacert.pem. The ECU has no system trust store
// (/etc/ssl/certs is absent), so the proxy ships its own roots rather than
// relying on x509.SystemCertPool, which is empty on that box. Refresh by
// re-fetching the bundle; provenance is recorded at the top of cacert.pem.
//
//go:embed cacert.pem
var caBundle []byte

// certPool builds the root pool used to verify the upstream feed: the
// vendored Mozilla roots, plus any additional PEM roots from extraCAFile
// (for a privately-hosted or self-signed feed). An empty extraCAFile adds
// nothing. It is an error if the vendored bundle fails to load or the extra
// file is given but unreadable/unparseable, so trust is never silently empty.
func certPool(extraCAFile string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBundle) {
		return nil, fmt.Errorf("tlsproxy: embedded CA bundle failed to parse")
	}
	if extraCAFile != "" {
		pem, err := os.ReadFile(extraCAFile)
		if err != nil {
			return nil, fmt.Errorf("tlsproxy: read extra CA file %s: %w", extraCAFile, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("tlsproxy: extra CA file %s has no usable certificates", extraCAFile)
		}
	}
	return pool, nil
}
