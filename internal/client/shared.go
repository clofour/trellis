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
	client *http.Client
}

func (c *client) request(ctx context.Context, method string, url string, requestData any, responseData any) error {
	var requestBody *bytes.Reader
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

	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("executing request %s: %w", url, err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if checkStatusCode(response.StatusCode) {
		return fmt.Errorf("status %d", response.StatusCode)
	}

	if responseData != nil {
		err = json.Unmarshal(responseBody, &responseData)
		if err != nil {
			return fmt.Errorf("unmarshal json: %w", err)
		}
	}

	return nil
}

func checkStatusCode(statusCode int) bool {
	return statusCode < 200 || statusCode >= 300
}
