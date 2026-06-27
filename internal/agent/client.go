package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/yazmeyaa/hosthalla/internal/host"
)

type Client struct {
	config *AgentConfig
}

func NewClient(config *AgentConfig) *Client {
	return &Client{config}
}

type HeartbeatResponse struct {
	Version int `json:"version"`
}

func (c *Client) SendHeartbeat(ctx context.Context) (*HeartbeatResponse, error) {
	u := url.URL{
		Scheme: c.config.Connection.Scheme,
		Host:   c.config.Connection.Host,
		Path:   fmt.Sprintf("/api/v1/heartbeat"),
	}

	resp, err := c.sendRequest(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, buildUnexpectedStatusError("send heartbeat", resp)
	}

	var response HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode heartbeat response: %w", err)
	}

	return &response, nil
}

func (c *Client) SendMetrics(ctx context.Context, metric host.HostMetric) error {
	u := url.URL{
		Scheme: c.config.Connection.Scheme,
		Host:   c.config.Connection.Host,
		Path:   fmt.Sprintf("/api/v1/metrics"),
	}

	resp, err := c.sendRequest(ctx, http.MethodPost, u.String(), metric)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return buildUnexpectedStatusError("send metrics", resp)
	}

	return nil
}

func (c *Client) sendRequest(ctx context.Context, method string, path string, body any) (*http.Response, error) {
	var bodyReader *bytes.Reader
	if body == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.Connection.APIKey))
	req.Header.Set("Hosthalla-Agent-ID", c.config.AgentID.String())
	req.Header.Set("Hosthalla-Agent-Version", fmt.Sprintf("%d", c.config.Version))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hosthalla-agent")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func buildUnexpectedStatusError(operation string, resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		return fmt.Errorf("%s: unexpected status code: %s", operation, resp.Status)
	}
	return fmt.Errorf("%s: unexpected status code: %s (%s)", operation, resp.Status, bodyText)
}
