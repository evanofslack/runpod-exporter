package collector

import "fmt"

const maxErrorBodyLen = 500

// httpError formats a non-2xx response into an error carrying the status
// code and a truncated response body.
func httpError(statusCode int, body []byte) error {
	if len(body) > maxErrorBodyLen {
		body = body[:maxErrorBodyLen]
	}
	return fmt.Errorf("unexpected status %d: %s", statusCode, body)
}
