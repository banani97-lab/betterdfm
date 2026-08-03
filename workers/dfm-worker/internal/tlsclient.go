package internal

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
)

// internalTransport returns an http.Transport for calling the gerbonara sidecar.
// In GovCloud the sidecar serves HTTPS (self-signed cert) so worker->gerbonara
// traffic is encrypted in transit (NIST 800-171 3.13.8). Endpoint authentication
// is provided by network isolation: only the worker's security group can reach
// the sidecar's port, so certificate verification is intentionally skipped while
// transit stays encrypted. Cloned from DefaultTransport to keep pooling/timeouts.
func internalTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	if strings.HasPrefix(strings.ToLower(os.Getenv("AWS_REGION")), "us-gov-") {
		t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // internal, SG-isolated; encrypt-only
	}
	return t
}
