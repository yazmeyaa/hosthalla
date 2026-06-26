package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *Client) SendHeartbeat() (*HeartbeatResponse, error) {
	u := url.URL{
		Scheme: c.config.Connection.Scheme,
		Host:   c.config.Connection.Host,
		Path:   fmt.Sprintf("/api/v1/heartbeat"),
	}

	resp, err := c.sendRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *Client) SendMetrics() error {
	return nil
}

func (c *Client) sendRequest(method string, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, path, body)
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
