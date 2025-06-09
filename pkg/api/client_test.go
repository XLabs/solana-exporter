package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/stretchr/testify/assert"
)

func init() {
	slog.Init()
}

func TestClient_GetMinRequiredVersion(t *testing.T) {
	tests := []struct {
		name         string
		cluster      string
		mockJSON     string
		currentEpoch int
		wantErr      bool
		wantErrMsg   string
		want         string
		wantEpoch    int
	}{
		{
			name:    "valid mainnet response with matching epoch",
			cluster: "mainnet-beta",
			mockJSON: `{
				"data": [
					{
						"cluster": "mainnet-beta",
						"epoch": 796,
						"agave_min_version": "2.2.14",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20214",
						"inherited_from_prev_epoch": true
					},
					{
						"cluster": "mainnet-beta",
						"epoch": 797,
						"agave_min_version": "2.2.15",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20215",
						"inherited_from_prev_epoch": false
					}
				]
			}`,
			currentEpoch: 797,
			want:         "2.2.15",
			wantEpoch:    797,
		},
		{
			name:    "valid mainnet response with fallback to first entry",
			cluster: "mainnet-beta",
			mockJSON: `{
				"data": [
					{
						"cluster": "mainnet-beta",
						"epoch": 796,
						"agave_min_version": "2.2.14",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20214",
						"inherited_from_prev_epoch": true
					}
				]
			}`,
			currentEpoch: 797,
			want:         "2.2.14",
			wantEpoch:    796,
		},
		{
			name:    "valid testnet response",
			cluster: "testnet",
			mockJSON: `{
				"data": [
					{
						"cluster": "testnet",
						"epoch": 797,
						"agave_min_version": "2.1.6",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20214",
						"inherited_from_prev_epoch": true
					}
				]
			}`,
			currentEpoch: 797,
			want:         "2.1.6",
			wantEpoch:    797,
		},
		{
			name:         "invalid json response",
			cluster:      "mainnet-beta",
			mockJSON:     `{"invalid": "json"`,
			currentEpoch: 797,
			wantErr:      true,
			wantErrMsg:   "failed to decode response",
		},
		{
			name:         "empty data array",
			cluster:      "mainnet-beta",
			mockJSON:     `{"data": []}`,
			currentEpoch: 797,
			wantErr:      true,
			wantErrMsg:   "no data found in response",
		},
		{
			name:         "missing agave_min_version",
			cluster:      "mainnet-beta",
			mockJSON:     `{"data": [{"cluster": "mainnet-beta", "epoch": 796, "firedancer_min_version": "0.503.20214"}]}`,
			currentEpoch: 797,
			wantErr:      true,
			wantErrMsg:   "agave_min_version not found in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "/api/epoch/required_versions", r.URL.Path)
				assert.Equal(t, tt.cluster, r.URL.Query().Get("cluster"))

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.mockJSON))
			}))
			defer server.Close()

			// Create mock RPC client
			mockServer, mockRPCClient := rpc.NewMockClient(t,
				map[string]any{
					"getEpochInfo": map[string]int{
						"epoch": tt.currentEpoch,
					},
				},
				nil,
				nil,
				nil,
				nil,
				nil,
			)
			defer mockServer.Close()

			// Create client with test server URL
			client := NewClient(mockRPCClient)
			client.baseURL = server.URL + "/api/epoch/required_versions"
			client.cacheTimeout = time.Hour

			// Test GetMinRequiredVersion
			got, gotCluster, gotEpoch, gotFiredancerVersion, err := client.GetMinRequiredVersion(context.Background(), tt.cluster)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.cluster, gotCluster)
			assert.Equal(t, tt.wantEpoch, gotEpoch)
			assert.NotEmpty(t, gotFiredancerVersion)

			// Test caching
			cachedVersion, cachedCluster, cachedEpoch, cachedFiredancerVersion, err := client.GetMinRequiredVersion(context.Background(), tt.cluster)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, cachedVersion)
			assert.Equal(t, tt.cluster, cachedCluster)
			assert.Equal(t, tt.wantEpoch, cachedEpoch)
			assert.NotEmpty(t, cachedFiredancerVersion)
		})
	}
}

func TestClient_GetNextEpochMinRequiredVersion(t *testing.T) {
	tests := []struct {
		name         string
		cluster      string
		mockJSON     string
		currentEpoch int
		wantErr      bool
		wantErrMsg   string
		want         string
		wantEpoch    int
	}{
		{
			name:    "valid mainnet response with next epoch",
			cluster: "mainnet-beta",
			mockJSON: `{
				"data": [
					{
						"cluster": "mainnet-beta",
						"epoch": 796,
						"agave_min_version": "2.2.14",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20214",
						"inherited_from_prev_epoch": true
					},
					{
						"cluster": "mainnet-beta",
						"epoch": 797,
						"agave_min_version": "2.2.15",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20215",
						"inherited_from_prev_epoch": false
					},
					{
						"cluster": "mainnet-beta",
						"epoch": 798,
						"agave_min_version": "2.2.16",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20216",
						"inherited_from_prev_epoch": false
					}
				]
			}`,
			currentEpoch: 797,
			want:         "2.2.16",
			wantEpoch:    798,
		},
		{
			name:    "no next epoch available - fallback to current epoch",
			cluster: "mainnet-beta",
			mockJSON: `{
				"data": [
					{
						"cluster": "mainnet-beta",
						"epoch": 797,
						"agave_min_version": "2.2.15",
						"agave_max_version": null,
						"firedancer_max_version": null,
						"firedancer_min_version": "0.503.20215",
						"inherited_from_prev_epoch": false
					}
				]
			}`,
			currentEpoch: 797,
			want:         "2.2.15",
			wantEpoch:    797,
		},
		{
			name:         "empty data array",
			cluster:      "mainnet-beta",
			mockJSON:     `{"data": []}`,
			currentEpoch: 797,
			wantErr:      true,
			wantErrMsg:   "no data found in response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				assert.Equal(t, "/api/epoch/required_versions", r.URL.Path)
				assert.Equal(t, tt.cluster, r.URL.Query().Get("cluster"))

				// Send response
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.mockJSON))
			}))
			defer server.Close()

			// Create mock RPC client
			mockServer, mockRPCClient := rpc.NewMockClient(t,
				map[string]any{
					"getEpochInfo": map[string]int{
						"epoch": tt.currentEpoch,
					},
				},
				nil,
				nil,
				nil,
				nil,
				nil,
			)
			defer mockServer.Close()

			// Create client with test server URL
			client := NewClient(mockRPCClient)
			client.baseURL = server.URL + "/api/epoch/required_versions"
			client.cacheTimeout = time.Hour

			// Test GetNextEpochMinRequiredVersion
			got, gotCluster, gotEpoch, gotFiredancerVersion, err := client.GetNextEpochMinRequiredVersion(context.Background(), tt.cluster)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.cluster, gotCluster)
			assert.Equal(t, tt.wantEpoch, gotEpoch)
			assert.NotEmpty(t, gotFiredancerVersion)

			// Test caching
			cachedVersion, cachedCluster, cachedEpoch, cachedFiredancerVersion, err := client.GetNextEpochMinRequiredVersion(context.Background(), tt.cluster)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, cachedVersion)
			assert.Equal(t, tt.cluster, cachedCluster)
			assert.Equal(t, tt.wantEpoch, cachedEpoch)
			assert.NotEmpty(t, cachedFiredancerVersion)
		})
	}
}
