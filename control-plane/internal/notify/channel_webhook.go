package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// deliverWebhook POSTs the event as JSON. This is the escape hatch channel: whatever
// the operator has that speaks HTTP gets the raw event and can do what it likes.
//
// An optional auth_header config value is sent verbatim as the Authorization header,
// which is what most self-hosted receivers want and is why that key is in the secret
// list.
func (n *Notifier) deliverWebhook(ctx context.Context, t Target, ev RunFailedEvent) error {
	if t.URL == "" {
		return errors.New("webhook target has no url")
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if h := t.Config["auth_header"]; h != "" {
		req.Header.Set("Authorization", h)
	}
	return n.doRequest(req)
}

// doRequest performs the call and turns a non-2xx into an error carrying the first
// part of the response body. Delivery failures are shown to the operator in the UI, so
// "HTTP 403" alone is not enough to act on.
func (n *Notifier) doRequest(req *http.Request) error {
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return nil
}

const userAgent = "croncompose-notifier/2"
