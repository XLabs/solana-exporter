package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/asymmetric-research/solana-exporter/pkg/api"
	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

type (
	Simulator struct {
		Server *rpc.MockServer

		Slot             int
		BlockHeight      int
		Epoch            int
		TransactionCount int

		// constants for the simulator
		SlotTime                time.Duration
		EpochSize               int
		LeaderSchedule          map[string][]int
		Nodekeys                []string
		Votekeys                []string
		FeeRewardLamports       int
		InflationRewardLamports int
		LastVoteDistances       map[string]int
		RootSlotDistances       map[string]int
	}
)

func NewSimulator(t *testing.T, slot int) (*Simulator, *rpc.Client) {
	nodekeys := []string{"aaa", "bbb", "ccc"}
	votekeys := []string{"AAA", "BBB", "CCC"}
	feeRewardLamports, inflationRewardLamports := 10, 10

	validatorInfos := make(map[string]rpc.MockValidatorInfo)
	for i, nodekey := range nodekeys {
		validatorInfos[nodekey] = rpc.MockValidatorInfo{
			Votekey:    votekeys[i],
			Stake:      1_000_000,
			Delinquent: false,
		}
	}
	leaderSchedule := map[string][]int{
		"aaa": {0, 1, 2, 3, 12, 13, 14, 15},
		"bbb": {4, 5, 6, 7, 16, 17, 18, 19},
		"ccc": {8, 9, 10, 11, 20, 21, 22, 23},
	}
	mockServer, client := rpc.NewMockClient(t,
		map[string]any{
			"getVersion":        map[string]string{"solana-core": "v1.0.0"},
			"getIdentity":       map[string]string{"identity": "testIdentity"},
			"getLeaderSchedule": leaderSchedule,
			"getHealth":         "ok",
			"getGenesisHash":    rpc.MainnetGenesisHash,
		},
		nil,
		map[string]int{
			"aaa": 1 * rpc.LamportsInSol,
			"bbb": 2 * rpc.LamportsInSol,
			"ccc": 3 * rpc.LamportsInSol,
			"AAA": 4 * rpc.LamportsInSol,
			"BBB": 5 * rpc.LamportsInSol,
			"CCC": 6 * rpc.LamportsInSol,
		},
		map[string]int{
			"AAA": inflationRewardLamports,
			"BBB": inflationRewardLamports,
			"CCC": inflationRewardLamports,
		},
		nil,
		validatorInfos,
	)
	simulator := Simulator{
		Slot:                    0,
		Server:                  mockServer,
		EpochSize:               24,
		SlotTime:                100 * time.Millisecond,
		LeaderSchedule:          leaderSchedule,
		Nodekeys:                nodekeys,
		Votekeys:                votekeys,
		InflationRewardLamports: inflationRewardLamports,
		FeeRewardLamports:       feeRewardLamports,
		LastVoteDistances:       map[string]int{"aaa": 1, "bbb": 2, "ccc": 3},
		RootSlotDistances:       map[string]int{"aaa": 4, "bbb": 5, "ccc": 6},
	}
	simulator.PopulateSlot(0)
	if slot > 0 {
		for {
			simulator.Slot++
			simulator.PopulateSlot(simulator.Slot)
			if simulator.Slot == slot {
				break
			}
		}
	}

	return &simulator, client
}

func (c *Simulator) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			c.Slot++
			c.PopulateSlot(c.Slot)
			// add 5% noise to the slot time:
			noiseRange := float64(c.SlotTime) * 0.05
			noise := (rand.Float64()*2 - 1) * noiseRange
			time.Sleep(c.SlotTime + time.Duration(noise))
		}
	}
}

func (c *Simulator) getLeader() string {
	index := c.Slot % c.EpochSize
	for leader, slots := range c.LeaderSchedule {
		if slices.Contains(slots, index) {
			return leader
		}
	}
	panic(fmt.Sprintf("leader not found at slot %d", c.Slot))
}

func (c *Simulator) PopulateSlot(slot int) {
	leader := c.getLeader()

	var block *rpc.MockBlockInfo
	// every 4th slot is skipped
	if slot%4 != 3 {
		c.BlockHeight++
		// only add some transactions if a block was produced
		transactions := [][]string{
			{"aaa", "bbb", "ccc"},
			{"xxx", "yyy", "zzz"},
		}
		// assume all validators voted
		for _, nodekey := range c.Nodekeys {
			transactions = append(transactions, []string{nodekey, strings.ToUpper(nodekey), VoteProgram})
			info := c.Server.GetValidatorInfo(nodekey)
			info.LastVote = max(0, slot-c.LastVoteDistances[nodekey])
			info.RootSlot = max(0, slot-c.RootSlotDistances[nodekey])
			c.Server.SetOpt(rpc.ValidatorInfoOpt, nodekey, info)
		}

		c.TransactionCount += len(transactions)
		block = &rpc.MockBlockInfo{Fee: c.FeeRewardLamports, Transactions: transactions}
	}
	// add slot info:
	c.Server.SetOpt(rpc.SlotInfosOpt, slot, rpc.MockSlotInfo{Leader: leader, Block: block})

	// now update the server:
	c.Epoch = int(math.Floor(float64(slot) / float64(c.EpochSize)))
	c.Server.SetOpt(
		rpc.EasyResultsOpt,
		"getSlot",
		slot,
	)
	c.Server.SetOpt(
		rpc.EasyResultsOpt,
		"getEpochInfo",
		map[string]int{
			"absoluteSlot":     slot,
			"blockHeight":      c.BlockHeight,
			"epoch":            c.Epoch,
			"slotIndex":        slot % c.EpochSize,
			"slotsInEpoch":     c.EpochSize,
			"transactionCount": c.TransactionCount,
		},
	)
	c.Server.SetOpt(
		rpc.EasyResultsOpt,
		"minimumLedgerSlot",
		int(math.Max(0, float64(slot-c.EpochSize))),
	)
	c.Server.SetOpt(
		rpc.EasyResultsOpt,
		"getFirstAvailableBlock",
		int(math.Max(0, float64(slot-c.EpochSize))),
	)
}

func newTestConfig(simulator *Simulator, fast bool) *ExporterConfig {
	pace := time.Duration(100) * time.Second
	if fast {
		pace = time.Duration(500) * time.Millisecond
	}
	config := ExporterConfig{
		HttpTimeout:                      time.Second * time.Duration(1),
		RpcUrl:                           simulator.Server.URL(),
		ListenAddress:                    ":8080",
		NodeKeys:                         simulator.Nodekeys,
		VoteKeys:                         simulator.Votekeys,
		BalanceAddresses:                 nil,
		ComprehensiveSlotTracking:        true,
		ComprehensiveVoteAccountTracking: true,
		MonitorBlockSizes:                true,
		LightMode:                        false,
		SlotPace:                         pace,
		ActiveIdentity:                   simulator.Nodekeys[0],
		// we need to set the epoch cleanup time to long enough such that we can test that the final state for the
		// previous epoch is correct before cleaning it. Ideally I would like a better way of doing this than simply
		// "waiting long enough", but this should do for now
		EpochCleanupTime: 5 * time.Second,
	}
	return &config
}

func TestSolanaCollector(t *testing.T) {
	simulator, client := NewSimulator(t, 35)
	simulator.Server.SetOpt(rpc.EasyResultsOpt, "getGenesisHash", rpc.MainnetGenesisHash)

	mock := api.NewMockClient()
	mock.SetMinRequiredVersion("2.0.20", "1.0.0")

	collector := NewSolanaCollector(client, mock.Client, newTestConfig(simulator, false))
	prometheus.NewPedanticRegistry().MustRegister(collector)

	stake := float64(1_000_000) / rpc.LamportsInSol

	testCases := []collectionTest{
		collector.ValidatorActiveStake.makeCollectionTest(
			NewLV(stake, "aaa", "AAA"),
			NewLV(stake, "bbb", "BBB"),
			NewLV(stake, "ccc", "CCC"),
		),
		collector.ClusterActiveStake.makeCollectionTest(
			NewLV(3 * stake),
		),
		collector.ValidatorLastVote.makeCollectionTest(
			NewLV(33, "aaa", "AAA"),
			NewLV(32, "bbb", "BBB"),
			NewLV(31, "ccc", "CCC"),
		),
		collector.ClusterLastVote.makeCollectionTest(
			NewLV(33),
		),
		collector.ValidatorRootSlot.makeCollectionTest(
			NewLV(30, "aaa", "AAA"),
			NewLV(29, "bbb", "BBB"),
			NewLV(28, "ccc", "CCC"),
		),
		collector.ClusterRootSlot.makeCollectionTest(
			NewLV(30),
		),
		collector.ValidatorDelinquent.makeCollectionTest(
			NewLV(0, "aaa", "AAA"),
			NewLV(0, "bbb", "BBB"),
			NewLV(0, "ccc", "CCC"),
		),
		collector.ClusterValidatorCount.makeCollectionTest(
			NewLV(3, StateCurrent),
			NewLV(0, StateDelinquent),
		),
		collector.NodeVersion.makeCollectionTest(
			NewLV(1, "0", "v1.0.0"),
		),
		collector.NodeIdentity.makeCollectionTest(
			NewLV(1, "testIdentity"),
		),
		collector.NodeIsActive.makeCollectionTest(
			NewLV(0, "testIdentity"),
		),
		collector.NodeIsHealthy.makeCollectionTest(
			NewLV(1),
		),
		collector.NodeNumSlotsBehind.makeCollectionTest(
			NewLV(0),
		),
		collector.AccountBalances.makeCollectionTest(
			NewLV(4, "AAA"),
			NewLV(5, "BBB"),
			NewLV(6, "CCC"),
			NewLV(1, "aaa"),
			NewLV(2, "bbb"),
			NewLV(3, "ccc"),
		),
		collector.NodeMinimumLedgerSlot.makeCollectionTest(
			NewLV(11),
		),
		collector.NodeFirstAvailableBlock.makeCollectionTest(
			NewLV(11),
		),
		collector.FoundationMinRequiredVersion.makeCollectionTest(
			NewLV(1, "2.0.20", "mainnet-beta", "0", "1.0.0"),
		),
	}

	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			err := testutil.CollectAndCompare(collector, bytes.NewBufferString(test.ExpectedResponse), test.Name)
			assert.NoErrorf(t, err, "unexpected collecting result for %s: \n%s", test.Name, err)
		})
	}
}

func TestSolanaCollector_collectHealth(t *testing.T) {
	simulator, client := NewSimulator(t, 0)
	simulator.Server.SetOpt(rpc.EasyResultsOpt, "getGenesisHash", rpc.MainnetGenesisHash)

	mock := api.NewMockClient()
	mock.SetMinRequiredVersion("2.0.20", "1.0.0")

	collector := NewSolanaCollector(client, mock.Client, newTestConfig(simulator, false))
	prometheus.NewPedanticRegistry().MustRegister(collector)

	t.Run("healthy", func(t *testing.T) {
		testCases := []collectionTest{
			collector.NodeIsHealthy.makeCollectionTest(NewLV(1)),
			collector.NodeNumSlotsBehind.makeCollectionTest(NewLV(0)),
		}

		for _, test := range testCases {
			t.Run(test.Name, func(t *testing.T) {
				err := testutil.CollectAndCompare(collector, bytes.NewBufferString(test.ExpectedResponse), test.Name)
				assert.NoErrorf(t, err, "unexpected collecting result for %s: \n%s", test.Name, err)
			})
		}
	})

	getHealthErr := rpc.Error{
		Code:    rpc.NodeUnhealthyCode,
		Method:  "getHealth",
		Message: "Node is unhealthy",
		Data:    map[string]any{"numSlotsBehind": 42},
	}

	// TODO: when I try test the generic case, it fails because of the error emitted to the
	//  solana_node_num_slots_behind metric
	t.Run("unhealthy", func(t *testing.T) {
		simulator.Server.SetOpt(rpc.EasyErrorsOpt, "getHealth", getHealthErr)

		testCases := []collectionTest{
			collector.NodeIsHealthy.makeCollectionTest(NewLV(0)),
		}
		for _, test := range testCases {
			t.Run(test.Name, func(t *testing.T) {
				err := testutil.CollectAndCompare(collector, bytes.NewBufferString(test.ExpectedResponse), test.Name)
				assert.NoErrorf(t, err, "unexpected collecting result for %s: \n%s", test.Name, err)
			})
		}
	})
}

func TestSolanaCollector_NodeIsOutdated(t *testing.T) {
	tests := []struct {
		name           string
		isFiredancer   bool
		version        string
		agaveVer       string
		firedancerVer  string
		expectedOutput string
	}{
		{
			name:          "firedancer outdated",
			isFiredancer:  true,
			version:       "0.9.0",
			agaveVer:      "1.0.0",
			firedancerVer: "1.0.0",
			expectedOutput: `
# HELP solana_node_outdated Whether the node is running a version below the required minimum for Firedancer
# TYPE solana_node_outdated gauge
solana_node_outdated{cluster="mainnet-beta",is_firedancer="1",required_version="1.0.0",version="0.9.0"} 1
`,
		},
		{
			name:          "firedancer up-to-date",
			isFiredancer:  true,
			version:       "1.2.0",
			agaveVer:      "1.0.0",
			firedancerVer: "1.0.0",
			expectedOutput: `
# HELP solana_node_outdated Whether the node is running a version below the required minimum for Firedancer
# TYPE solana_node_outdated gauge
solana_node_outdated{cluster="mainnet-beta",is_firedancer="1",required_version="1.0.0",version="1.2.0"} 0
`,
		},
		{
			name:          "not firedancer outdated",
			isFiredancer:  false,
			version:       "0.9.0",
			agaveVer:      "1.0.0",
			firedancerVer: "1.0.0",
			expectedOutput: `
# HELP solana_node_outdated Whether the node is running a version below the required minimum for Firedancer
# TYPE solana_node_outdated gauge
solana_node_outdated{cluster="mainnet-beta",is_firedancer="0",required_version="1.0.0",version="0.9.0"} 1
`,
		},
		{
			name:          "not firedancer up-to-date",
			isFiredancer:  false,
			version:       "1.2.0",
			agaveVer:      "1.0.0",
			firedancerVer: "1.0.0",
			expectedOutput: `
# HELP solana_node_outdated Whether the node is running a version below the required minimum for Firedancer
# TYPE solana_node_outdated gauge
solana_node_outdated{cluster="mainnet-beta",is_firedancer="0",required_version="1.0.0",version="1.2.0"} 0
`,
		},
		{
			name:          "different versions",
			isFiredancer:  false,
			version:       "1.1.0",
			agaveVer:      "1.0.0",
			firedancerVer: "2.0.0",
			expectedOutput: `
# HELP solana_node_outdated Whether the node is running a version below the required minimum for Firedancer
# TYPE solana_node_outdated gauge
solana_node_outdated{cluster="mainnet-beta",is_firedancer="0",required_version="1.0.0",version="1.1.0"} 0
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := rpc.NewMockClient(t,
				map[string]any{
					"getVersion":            map[string]string{"solana-core": tt.version},
					"getGenesisHash":        rpc.MainnetGenesisHash,
					"getHealth":             "ok",
					"getIdentity":           map[string]string{"identity": "testIdentity"},
					"minimumLedgerSlot":     0,
					"getFirstAvailableBlock": 0,
					"getVoteAccounts": map[string]any{
						"current":    []any{},
						"delinquent": []any{},
					},
				},
				nil,
				nil,
				nil,
				nil,
				nil,
			)

			mock := api.NewMockClient()
			mock.SetMinRequiredVersion(tt.agaveVer, tt.firedancerVer)

			collector := NewSolanaCollector(client, mock.Client, &ExporterConfig{})
			collector.isFiredancer = tt.isFiredancer

			if err := testutil.CollectAndCompare(collector, strings.NewReader(tt.expectedOutput), "solana_node_outdated"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
