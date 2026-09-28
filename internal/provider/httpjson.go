package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type StatusError struct {
	URL        string
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GET %s: unexpected status %d", e.URL, e.StatusCode)
}

func GetJSON(ctx context.Context, client *http.Client, url string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", url, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return &StatusError{URL: url, StatusCode: response.StatusCode}
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response from %s: %w", url, err)
	}
	return nil
}
