package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/kanywst/approval-hub/internal/config"
)

type brokerClient struct {
	url   string
	token string
}

func newClient(cfg config.Config) *brokerClient {
	return &brokerClient{
		url:   fmt.Sprintf("http://127.0.0.1:%d", cfg.Port),
		token: cfg.Token,
	}
}

func (c *brokerClient) request(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.url+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect broker at %s: %w", c.url, err)
	}
	return resp, nil
}

func loadConfig() (config.Config, error) {
	p, err := configPath()
	if err != nil {
		return config.Config{}, err
	}
	return config.Load(p)
}
