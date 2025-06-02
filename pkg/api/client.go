package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	// CacheTimeout defines how often to refresh the minimum required version (6 hours)
	CacheTimeout = 6 * time.Hour

	// SolanaEpochStatsAPI is the base URL for the Solana validators epoch stats API
	SolanaEpochStatsAPI = "https://api.solana.org/api/epoch/required_versions"
)

type Client struct {
	HttpClient http.Client
	baseURL    string
	cache      struct {
		agaveVersion      string
		firedancerVersion string
		lastCheck         time.Time
		epoch             int
	}
	mu sync.RWMutex
	// How often to refresh the cache
	cacheTimeout time.Duration
}

func NewClient() *Client {
	return &Client{
		HttpClient:   http.Client{},
		cacheTimeout: CacheTimeout,
		baseURL:      SolanaEpochStatsAPI,
	}
}

func (c *Client) GetMinRequiredVersion(ctx context.Context, cluster string) (string, string, int, string, error) {
	// Check cache first
	c.mu.RLock()
	if !c.cache.lastCheck.IsZero() && time.Since(c.cache.lastCheck) < c.cacheTimeout {
		version := c.cache.agaveVersion
		firedancerVersion := c.cache.firedancerVersion
		epoch := c.cache.epoch
		c.mu.RUnlock()
		return version, cluster, epoch, firedancerVersion, nil
	}
	c.mu.RUnlock()

	// Make API request
	url := fmt.Sprintf("%s?cluster=%s", c.baseURL, cluster)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", cluster, 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return "", cluster, 0, "", fmt.Errorf("failed to fetch min required version: %w", err)
	}
	defer resp.Body.Close()

	var stats ValidatorEpochStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return "", cluster, 0, "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Validate the response
	if len(stats.Data) == 0 {
		return "", cluster, 0, "", fmt.Errorf("no data found in response")
	}

	// Get the first element's agave_min_version and epoch
	agaveMinVersion := stats.Data[0].AgaveMinVersion
	if agaveMinVersion == "" {
		return "", cluster, 0, "", fmt.Errorf("agave_min_version not found in response")
	}

	firedancerMinVersion := stats.Data[0].FiredancerMinVersion
	epoch := stats.Data[0].Epoch

	// Update cache
	c.mu.Lock()
	c.cache.agaveVersion = agaveMinVersion
	c.cache.firedancerVersion = firedancerMinVersion
	c.cache.epoch = epoch
	c.cache.lastCheck = time.Now()
	c.mu.Unlock()

	return agaveMinVersion, cluster, epoch, firedancerMinVersion, nil
}
