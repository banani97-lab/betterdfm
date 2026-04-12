package internal

import (
	"log"
	"net/http"
	"time"
)

// WaitForSidecar blocks until the sidecar's health endpoint returns 200,
// retrying every 2 seconds up to maxWaitSec. This prevents the worker from
// pulling SQS jobs before the gerbonara sidecar has finished booting.
func WaitForSidecar(healthURL string, maxWaitSec int) {
	deadline := time.Now().Add(time.Duration(maxWaitSec) * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Printf("sidecar ready at %s", healthURL)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			log.Printf("WARNING: sidecar at %s not ready after %ds, proceeding anyway", healthURL, maxWaitSec)
			return
		}
		time.Sleep(2 * time.Second)
	}
}
