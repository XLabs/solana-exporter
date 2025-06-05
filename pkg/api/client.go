package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
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
	rpcClient  *rpc.Client
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

func NewClient(rpcClient *rpc.Client) *Client {
	return &Client{
		HttpClient:   http.Client{},
		cacheTimeout: CacheTimeout,
		baseURL:      SolanaEpochStatsAPI,
		rpcClient:    rpcClient,
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

	// Get the current epoch from the node
	epochInfo, err := c.rpcClient.GetEpochInfo(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", cluster, 0, "", fmt.Errorf("failed to get current epoch: %w", err)
	}

	// Find the entry that matches the current epoch
	var matchingEntry *struct {
		Cluster                string  `json:"cluster"`
		Epoch                  int     `json:"epoch"`
		AgaveMinVersion        string  `json:"agave_min_version"`
		AgaveMaxVersion        *string `json:"agave_max_version"`
		FiredancerMaxVersion   *string `json:"firedancer_max_version"`
		FiredancerMinVersion   string  `json:"firedancer_min_version"`
		InheritedFromPrevEpoch bool    `json:"inherited_from_prev_epoch"`
	}
	for i := range stats.Data {
		if stats.Data[i].Epoch == int(epochInfo.Epoch) {
			matchingEntry = &stats.Data[i]
			break
		}
	}

	// If no matching entry found, use the first entry as fallback
	if matchingEntry == nil {
		matchingEntry = &stats.Data[0]
	}

	agaveMinVersion := matchingEntry.AgaveMinVersion
	if agaveMinVersion == "" {
		return "", cluster, 0, "", fmt.Errorf("agave_min_version not found in response")
	}

	firedancerMinVersion := matchingEntry.FiredancerMinVersion
	epoch := matchingEntry.Epoch

	// Update cache
	c.mu.Lock()
	c.cache.agaveVersion = agaveMinVersion
	c.cache.firedancerVersion = firedancerMinVersion
	c.cache.epoch = epoch
	c.cache.lastCheck = time.Now()
	c.mu.Unlock()

	return agaveMinVersion, cluster, epoch, firedancerMinVersion, nil
}
