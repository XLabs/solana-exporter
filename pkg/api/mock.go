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
		cache: struct {
			agaveVersion          string
			firedancerVersion     string
			nextAgaveVersion      string
			nextFiredancerVersion string
			lastCheck             time.Time
			epoch                 int
			nextEpoch             int
		}{},
	}
	return mock
}

func (m *Client) SetMinRequiredVersion(agaveVersion, firedancerVersion string) {
	m.cache.agaveVersion = agaveVersion
	m.cache.firedancerVersion = firedancerVersion
	m.cache.epoch = 797 // Set a specific epoch value
	m.cache.lastCheck = time.Now()
}

func (m *Client) SetNextEpochMinRequiredVersion(agaveVersion, firedancerVersion string) {
	m.cache.nextAgaveVersion = agaveVersion
	m.cache.nextFiredancerVersion = firedancerVersion
	m.cache.nextEpoch = 798 // Set next epoch value
	m.cache.lastCheck = time.Now()
}

func (m *MockClient) GetMinRequiredVersion(ctx context.Context, cluster string) (string, string, int, string, error) {
	return m.cache.agaveVersion, cluster, m.cache.epoch, m.cache.firedancerVersion, nil
}

func (m *MockClient) GetNextEpochMinRequiredVersion(ctx context.Context, cluster string) (string, string, int, string, error) {
	return m.cache.nextAgaveVersion, cluster, m.cache.nextEpoch, m.cache.nextFiredancerVersion, nil
}
