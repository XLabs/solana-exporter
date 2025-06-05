package api

import (
	"context"
	"net/http"
	"time"
)

type MockClient struct {
	*Client
}

func NewMockClient() *Client {
	mock := &Client{
		HttpClient:   http.Client{},
		baseURL:      SolanaEpochStatsAPI,
		cacheTimeout: CacheTimeout,
	}
	return mock
}

func (m *Client) SetMinRequiredVersion(agaveVersion, firedancerVersion string) {
	m.cache.agaveVersion = agaveVersion
	m.cache.firedancerVersion = firedancerVersion
	m.cache.epoch = 797 // Set a specific epoch value
	m.cache.lastCheck = time.Now()
}

func (m *MockClient) GetMinRequiredVersion(ctx context.Context, cluster string) (string, string, int, string, error) {
	return m.cache.agaveVersion, cluster, 0, m.cache.firedancerVersion, nil
}
