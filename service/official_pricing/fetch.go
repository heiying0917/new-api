package official_pricing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ModelsDevURL is the official models.dev pricing endpoint. It is a fixed,
// trusted URL (admin-triggered), so no SSRF guard is required.
const ModelsDevURL = "https://models.dev/api.json"

const maxBodyBytes = 16 << 20 // 16MB safety cap (payload is ~2.4MB today)

// FetchModelsDev downloads the models.dev pricing payload with a small retry.
func FetchModelsDev(ctx context.Context, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, err := fetchOnce(ctx, client)
		if err == nil {
			return body, nil
		}
		lastErr = err
		time.Sleep(time.Duration(200*(1<<attempt)) * time.Millisecond)
	}
	return nil, lastErr
}

func fetchOnce(ctx context.Context, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsDevURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev returned %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
}
