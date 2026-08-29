package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type client struct {
	token  string
	tenant string
	client *http.Client
}

func (c *client) request(ctx context.Context, method string, url string, requestData any, responseData any) error {
	var requestBody io.Reader = http.NoBody
	if requestData != nil {
		requestBodyBytes, err := json.Marshal(requestData)
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		requestBody = bytes.NewReader(requestBodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return fmt.Errorf("constructing request %s: %w", url, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	if c.tenant != "" {
		request.Header.Set("X-Trellis-Tenant", c.tenant)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("executing request %s: %w", url, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if checkStatusCode(response.StatusCode) {
		message := string(bytes.TrimSpace(responseBody))
		if message == "" {
			return fmt.Errorf("status %d", response.StatusCode)
		}
		return fmt.Errorf("status %d: %s", response.StatusCode, message)
	}

	if responseData != nil {
		err = json.Unmarshal(responseBody, responseData)
		if err != nil {
			return fmt.Errorf("unmarshal json: %w", err)
		}
	}

	return nil
}

func (c *client) stream(ctx context.Context, url string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("constructing request %s: %w", url, err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if c.tenant != "" {
		request.Header.Set("X-Trellis-Tenant", c.tenant)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("executing request %s: %w", url, err)
	}
	if checkStatusCode(response.StatusCode) {
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("status %d: %s", response.StatusCode, bytes.TrimSpace(body))
	}
	return response.Body, nil
}

func checkStatusCode(statusCode int) bool {
	return statusCode < 200 || statusCode >= 300
}
