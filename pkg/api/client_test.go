package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClient_GetMinRequiredVersion(t *testing.T) {
	tests := []struct {
		name       string
		cluster    string
		mockJSON   string
		wantErr    bool
		wantErrMsg string
		want       string
		wantEpoch  int
	}{
		{
			name:    "valid mainnet response",
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
			want:      "2.2.14",
			wantEpoch: 796,
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
			want:      "2.1.6",
			wantEpoch: 797,
		},
		{
			name:       "invalid json response",
			cluster:    "mainnet-beta",
			mockJSON:   `{"invalid": "json"`,
			wantErr:    true,
			wantErrMsg: "failed to decode response",
		},
		{
			name:       "empty data array",
			cluster:    "mainnet-beta",
			mockJSON:   `{"data": []}`,
			wantErr:    true,
			wantErrMsg: "no data found in response",
		},
		{
			name:       "missing agave_min_version",
			cluster:    "mainnet-beta",
			mockJSON:   `{"data": [{"cluster": "mainnet-beta", "epoch": 796}]}`,
			wantErr:    true,
			wantErrMsg: "agave_min_version not found in response",
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

			// Create client with test server URL
			client := &Client{
				HttpClient:   http.Client{},
				baseURL:      server.URL + "/api/epoch/required_versions",
				cacheTimeout: time.Hour,
			}

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
