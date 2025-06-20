package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/asymmetric-research/solana-exporter/pkg/api"
	"github.com/asymmetric-research/solana-exporter/pkg/rpc"
	"github.com/asymmetric-research/solana-exporter/pkg/slog"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"slices"
)

const (
	SkipStatusLabel      = "status"
	StateLabel           = "state"
	NodekeyLabel         = "nodekey"
	VotekeyLabel         = "votekey"
	VersionLabel         = "version"
	IdentityLabel        = "identity"
	AddressLabel         = "address"
	EpochLabel           = "epoch"
	TransactionTypeLabel = "transaction_type"
	IsFiredancerLabel    = "is_firedancer"
	ClusterLabel         = "cluster"

	StatusSkipped = "skipped"
	StatusValid   = "valid"

	StateCurrent    = "current"
	StateDelinquent = "delinquent"

	TransactionTypeVote    = "vote"
	TransactionTypeNonVote = "non_vote"
)

type SolanaCollector struct {
	rpcClient *rpc.Client
	apiClient *api.Client
	logger    *zap.SugaredLogger

	config *ExporterConfig

	/// descriptors:
	ValidatorActiveStake         *GaugeDesc
	ClusterActiveStake           *GaugeDesc
	ValidatorLastVote            *GaugeDesc
	ClusterLastVote              *GaugeDesc
	ValidatorRootSlot            *GaugeDesc
	ClusterRootSlot              *GaugeDesc
	ValidatorDelinquent          *GaugeDesc
	ClusterValidatorCount        *GaugeDesc
	AccountBalances              *GaugeDesc
	NodeVersion                  *GaugeDesc
	NodeIsHealthy                *GaugeDesc
	NodeNumSlotsBehind           *GaugeDesc
	NodeMinimumLedgerSlot        *GaugeDesc
	NodeFirstAvailableBlock      *GaugeDesc
	NodeIdentity                 *GaugeDesc
	NodeIsActive                 *GaugeDesc
	FoundationMinRequiredVersion *GaugeDesc
	NodeIsOutdated               *GaugeDesc
	NodeNeedsUpdate              *GaugeDesc

	isFiredancer bool
}

func NewSolanaCollector(rpcClient *rpc.Client, config *ExporterConfig) *SolanaCollector {
	collector := &SolanaCollector{
		rpcClient: rpcClient,
		apiClient: api.NewClient(rpcClient),
		logger:    slog.Get(),
		config:    config,
		ValidatorActiveStake: NewGaugeDesc(
			"solana_validator_active_stake",
			fmt.Sprintf("Active stake (in SOL) per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterActiveStake: NewGaugeDesc(
			"solana_cluster_active_stake",
			"Total active stake (in SOL) of the cluster",
		),
		ValidatorLastVote: NewGaugeDesc(
			"solana_validator_last_vote",
			fmt.Sprintf("Last voted-on slot per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterLastVote: NewGaugeDesc(
			"solana_cluster_last_vote",
			"Most recent voted-on slot of the cluster",
		),
		ValidatorRootSlot: NewGaugeDesc(
			"solana_validator_root_slot",
			fmt.Sprintf("Root slot per validator (represented by %s and %s)", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterRootSlot: NewGaugeDesc(
			"solana_cluster_root_slot",
			"Max root slot of the cluster",
		),
		ValidatorDelinquent: NewGaugeDesc(
			"solana_validator_delinquent",
			fmt.Sprintf("Whether a validator (represented by %s and %s) is delinquent", VotekeyLabel, NodekeyLabel),
			VotekeyLabel, NodekeyLabel,
		),
		ClusterValidatorCount: NewGaugeDesc(
			"solana_cluster_validator_count",
			fmt.Sprintf(
				"Total number of validators in the cluster, grouped by %s ('%s' or '%s')",
				StateLabel, StateCurrent, StateDelinquent,
			),
			StateLabel,
		),
		AccountBalances: NewGaugeDesc(
			"solana_account_balance",
			fmt.Sprintf("Solana account balances, grouped by %s", AddressLabel),
			AddressLabel,
		),
		NodeVersion: NewGaugeDesc(
			"solana_node_version",
			"Node version of solana",
			VersionLabel, IsFiredancerLabel,
		),
		NodeIdentity: NewGaugeDesc(
			"solana_node_identity",
			"Node identity of solana",
			IdentityLabel,
		),
		NodeIsHealthy: NewGaugeDesc(
			"solana_node_is_healthy",
			"Whether the node is healthy",
		),
		NodeNumSlotsBehind: NewGaugeDesc(
			"solana_node_num_slots_behind",
			"The number of slots that the node is behind the latest cluster confirmed slot.",
		),
		NodeMinimumLedgerSlot: NewGaugeDesc(
			"solana_node_minimum_ledger_slot",
			"The lowest slot that the node has information about in its ledger.",
		),
		NodeFirstAvailableBlock: NewGaugeDesc(
			"solana_node_first_available_block",
			"The slot of the lowest confirmed block that has not been purged from the node's ledger.",
		),
		NodeIsActive: NewGaugeDesc(
			"solana_node_is_active",
			fmt.Sprintf("Whether the node is active and participating in consensus (using %s pubkey)", IdentityLabel),
			IdentityLabel,
		),
		FoundationMinRequiredVersion: NewGaugeDesc(
			"solana_foundation_min_required_version",
			"Minimum required Solana version for the solana foundation delegation program",
			"agave_min_version", "firedancer_min_version", ClusterLabel, EpochLabel,
		),
		NodeIsOutdated: NewGaugeDesc(
			"solana_node_is_outdated",
			"Whether the node is running a version below the required minimum for Firedancer",
			IsFiredancerLabel, VersionLabel, "required_version", ClusterLabel, EpochLabel,
		),
		NodeNeedsUpdate: NewGaugeDesc(
			"solana_node_needs_update",
			"Whether the node needs to be updated before the next epoch to remain compliant",
			IsFiredancerLabel, VersionLabel, "required_version", ClusterLabel, EpochLabel,
		),
	}
	return collector
}

func (c *SolanaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.NodeVersion.Desc
	ch <- c.NodeIdentity.Desc
	ch <- c.ValidatorActiveStake.Desc
	ch <- c.ClusterActiveStake.Desc
	ch <- c.ValidatorLastVote.Desc
	ch <- c.ClusterLastVote.Desc
	ch <- c.ValidatorRootSlot.Desc
	ch <- c.ClusterRootSlot.Desc
	ch <- c.ValidatorDelinquent.Desc
	ch <- c.ClusterValidatorCount.Desc
	ch <- c.AccountBalances.Desc
	ch <- c.NodeIsHealthy.Desc
	ch <- c.NodeNumSlotsBehind.Desc
	ch <- c.NodeMinimumLedgerSlot.Desc
	ch <- c.NodeFirstAvailableBlock.Desc
	ch <- c.NodeIsActive.Desc
	ch <- c.FoundationMinRequiredVersion.Desc
	ch <- c.NodeIsOutdated.Desc
	ch <- c.NodeNeedsUpdate.Desc
}

func (c *SolanaCollector) collectVoteAccounts(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping vote-accounts collection in light mode.")
		return
	}
	c.logger.Info("Collecting vote accounts...")
	voteAccounts, err := c.rpcClient.GetVoteAccounts(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		c.logger.Errorf("failed to get vote accounts: %v", err)
		ch <- c.ValidatorActiveStake.NewInvalidMetric(err)
		ch <- c.ClusterActiveStake.NewInvalidMetric(err)
		ch <- c.ValidatorLastVote.NewInvalidMetric(err)
		ch <- c.ClusterLastVote.NewInvalidMetric(err)
		ch <- c.ValidatorRootSlot.NewInvalidMetric(err)
		ch <- c.ClusterRootSlot.NewInvalidMetric(err)
		ch <- c.ValidatorDelinquent.NewInvalidMetric(err)
		ch <- c.ClusterValidatorCount.NewInvalidMetric(err)
		return
	}

	var (
		totalStake  float64
		maxLastVote float64
		maxRootSlot float64
	)
	for _, account := range append(voteAccounts.Current, voteAccounts.Delinquent...) {
		accounts := []string{account.VotePubkey, account.NodePubkey}
		stake, lastVote, rootSlot :=
			float64(account.ActivatedStake)/rpc.LamportsInSol,
			float64(account.LastVote),
			float64(account.RootSlot)

		if slices.Contains(c.config.NodeKeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
			ch <- c.ValidatorActiveStake.MustNewConstMetric(stake, accounts...)
			ch <- c.ValidatorLastVote.MustNewConstMetric(lastVote, accounts...)
			ch <- c.ValidatorRootSlot.MustNewConstMetric(rootSlot, accounts...)
		}

		totalStake += stake
		maxLastVote = max(maxLastVote, lastVote)
		maxRootSlot = max(maxRootSlot, rootSlot)
	}

	{
		for _, account := range voteAccounts.Current {
			if slices.Contains(c.config.NodeKeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
				ch <- c.ValidatorDelinquent.MustNewConstMetric(0, account.VotePubkey, account.NodePubkey)
			}
		}
		for _, account := range voteAccounts.Delinquent {
			if slices.Contains(c.config.NodeKeys, account.NodePubkey) || c.config.ComprehensiveVoteAccountTracking {
				ch <- c.ValidatorDelinquent.MustNewConstMetric(1, account.VotePubkey, account.NodePubkey)
			}
		}
	}

	ch <- c.ClusterActiveStake.MustNewConstMetric(totalStake)
	ch <- c.ClusterLastVote.MustNewConstMetric(maxLastVote)
	ch <- c.ClusterRootSlot.MustNewConstMetric(maxRootSlot)
	ch <- c.ClusterValidatorCount.MustNewConstMetric(float64(len(voteAccounts.Current)), StateCurrent)
	ch <- c.ClusterValidatorCount.MustNewConstMetric(float64(len(voteAccounts.Delinquent)), StateDelinquent)

	c.logger.Info("Vote accounts collected.")
}

func (c *SolanaCollector) collectVersion(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting version...")
	version, err := c.rpcClient.GetVersion(ctx)
	if err != nil {
		c.logger.Errorf("failed to get version: %v", err)
		ch <- c.NodeVersion.NewInvalidMetric(err)
		return
	}

	// Use the isFiredancer field that was set in Collect
	isFiredancer := "0"
	if c.isFiredancer {
		isFiredancer = "1"
	}

	ch <- c.NodeVersion.MustNewConstMetric(1, version, isFiredancer)
	c.logger.Info("Version collected.")
}

func (c *SolanaCollector) collectIdentity(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting identity...")
	identity, err := c.rpcClient.GetIdentity(ctx)
	if err != nil {
		c.logger.Errorf("failed to get identity: %v", err)
		ch <- c.NodeIdentity.NewInvalidMetric(err)
		return
	}

	if c.config.ActiveIdentity != "" {
		isActive := 0
		if c.config.ActiveIdentity == identity {
			isActive = 1
		}
		ch <- c.NodeIsActive.MustNewConstMetric(float64(isActive), identity)
		c.logger.Info("NodeIsActive collected.")
	}

	ch <- c.NodeIdentity.MustNewConstMetric(1, identity)
	c.logger.Info("Identity collected.")
}

func (c *SolanaCollector) collectMinimumLedgerSlot(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting minimum ledger slot...")
	slot, err := c.rpcClient.GetMinimumLedgerSlot(ctx)
	if err != nil {
		c.logger.Errorf("failed to get minimum lidger slot: %v", err)
		ch <- c.NodeMinimumLedgerSlot.NewInvalidMetric(err)
		return
	}

	ch <- c.NodeMinimumLedgerSlot.MustNewConstMetric(float64(slot))
	c.logger.Info("Minimum ledger slot collected.")
}

func (c *SolanaCollector) collectFirstAvailableBlock(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting first available block...")
	block, err := c.rpcClient.GetFirstAvailableBlock(ctx)
	if err != nil {
		c.logger.Errorf("failed to get first available block: %v", err)
		ch <- c.NodeFirstAvailableBlock.NewInvalidMetric(err)
		return
	}

	ch <- c.NodeFirstAvailableBlock.MustNewConstMetric(float64(block))
	c.logger.Info("First available block collected.")
}

func (c *SolanaCollector) collectBalances(ctx context.Context, ch chan<- prometheus.Metric) {
	if c.config.LightMode {
		c.logger.Debug("Skipping balance collection in light mode.")
		return
	}
	c.logger.Info("Collecting balances...")
	balances, err := FetchBalances(
		ctx, c.rpcClient, CombineUnique(c.config.BalanceAddresses, c.config.NodeKeys, c.config.VoteKeys),
	)
	if err != nil {
		c.logger.Errorf("failed to get balances: %v", err)
		ch <- c.AccountBalances.NewInvalidMetric(err)
		return
	}

	for address, balance := range balances {
		ch <- c.AccountBalances.MustNewConstMetric(balance, address)
	}
	c.logger.Info("Balances collected.")
}

func (c *SolanaCollector) collectHealth(ctx context.Context, ch chan<- prometheus.Metric) {
	c.logger.Info("Collecting health...")

	health, err := c.rpcClient.GetHealth(ctx)
	isHealthy, isHealthyErr, numSlotsBehind, numSlotsBehindErr := ExtractHealthAndNumSlotsBehind(health, err)
	if isHealthyErr != nil {
		c.logger.Errorf("failed to determine node health: %v", isHealthyErr)
		ch <- c.NodeIsHealthy.NewInvalidMetric(err)
	} else {
		ch <- c.NodeIsHealthy.MustNewConstMetric(BoolToFloat64(isHealthy))
	}

	if numSlotsBehindErr != nil {
		c.logger.Errorf("failed to determine number of slots behind: %v", numSlotsBehindErr)
		ch <- c.NodeNumSlotsBehind.NewInvalidMetric(numSlotsBehindErr)
	} else {
		ch <- c.NodeNumSlotsBehind.MustNewConstMetric(float64(numSlotsBehind))
	}

	c.logger.Info("Health collected.")
	return
}

func compareVersions(a, b string) int {
	// Compare dot-separated version strings, e.g., "0.503.20214"
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		var aVal, bVal int
		if i < len(aParts) {
			aVal, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bVal, _ = strconv.Atoi(bParts[i])
		}
		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
	}
	return 0
}

func (c *SolanaCollector) collectNodeIsOutdated(ch chan<- prometheus.Metric) {
	version, err := c.rpcClient.GetVersion(context.Background())
	if err != nil {
		c.logger.Errorw("failed to get version", "error", err)
		return
	}

	cluster := "mainnet-beta" // Default to mainnet-beta
	genesisHash, err := c.rpcClient.GetGenesisHash(context.Background())
	if err == nil {
		cluster, err = rpc.GetClusterFromGenesisHash(genesisHash)
		if err != nil {
			c.logger.Errorw("failed to get cluster from genesis hash", "error", err)
		}
	}

	agaveMinVersion, _, epoch, firedancerMinVersion, err := c.apiClient.GetMinRequiredVersion(context.Background(), cluster)
	if err != nil {
		c.logger.Errorw("failed to get required version", "error", err)
		return
	}

	// Choose the appropriate minimum version based on whether the node is running Firedancer
	requiredVersion := agaveMinVersion
	if c.isFiredancer {
		requiredVersion = firedancerMinVersion
	}

	// Compare versions and determine if the node is outdated
	isOutdated := compareVersions(version, requiredVersion) < 0
	c.logger.Infow("node version check",
		"current_version", version,
		"required_version", requiredVersion,
		"is_outdated", isOutdated,
		"cluster", cluster,
		"is_firedancer", c.isFiredancer,
		"agave_min_version", agaveMinVersion,
		"firedancer_min_version", firedancerMinVersion,
		"epoch", epoch,
	)

	isFiredancerStr := "0"
	if c.isFiredancer {
		isFiredancerStr = "1"
	}

	ch <- c.NodeIsOutdated.MustNewConstMetric(
		BoolToFloat64(isOutdated),
		isFiredancerStr,
		version,
		requiredVersion,
		cluster,
		fmt.Sprintf("%d", epoch),
	)
}

func (c *SolanaCollector) collectNodeNeedsUpdate(ch chan<- prometheus.Metric) {
	version, err := c.rpcClient.GetVersion(context.Background())
	if err != nil {
		c.logger.Errorw("failed to get version", "error", err)
		return
	}
	c.logger.Infow("current node version", "version", version)

	cluster := "mainnet-beta" // Default to mainnet-beta
	genesisHash, err := c.rpcClient.GetGenesisHash(context.Background())
	if err == nil {
		cluster, err = rpc.GetClusterFromGenesisHash(genesisHash)
		if err != nil {
			c.logger.Errorw("failed to get cluster from genesis hash", "error", err)
		}
	}
	c.logger.Infow("detected cluster", "cluster", cluster)

	// Get next epoch version requirements
	nextAgaveMinVersion, _, nextEpoch, nextFiredancerMinVersion, err := c.apiClient.GetNextEpochMinRequiredVersion(context.Background(), cluster)
	if err != nil {
		c.logger.Errorw("failed to get next epoch required version", "error", err)
		return
	}
	c.logger.Infow("next epoch version requirements",
		"next_agave_min_version", nextAgaveMinVersion,
		"next_firedancer_min_version", nextFiredancerMinVersion,
		"next_epoch", nextEpoch,
	)

	// Choose the appropriate minimum version based on whether the node is running Firedancer
	nextRequiredVersion := nextAgaveMinVersion
	if c.isFiredancer {
		nextRequiredVersion = nextFiredancerMinVersion
	}
	c.logger.Infow("selected required version",
		"is_firedancer", c.isFiredancer,
		"next_required_version", nextRequiredVersion,
	)

	// Compare versions and determine if the node needs an update for the next epoch
	needsUpdate := compareVersions(version, nextRequiredVersion) < 0
	c.logger.Infow("node next epoch version check",
		"current_version", version,
		"next_epoch_required_version", nextRequiredVersion,
		"needs_update", needsUpdate,
		"cluster", cluster,
		"is_firedancer", c.isFiredancer,
		"next_epoch", nextEpoch,
		"next_agave_min_version", nextAgaveMinVersion,
		"next_firedancer_min_version", nextFiredancerMinVersion,
	)

	isFiredancerStr := "0"
	if c.isFiredancer {
		isFiredancerStr = "1"
	}

	ch <- c.NodeNeedsUpdate.MustNewConstMetric(
		BoolToFloat64(needsUpdate),
		isFiredancerStr,
		version,
		nextRequiredVersion,
		cluster,
		fmt.Sprintf("%d", nextEpoch),
	)
}

func (c *SolanaCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Info("========== BEGIN COLLECTION ==========")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.collectHealth(ctx, ch)
	c.collectMinimumLedgerSlot(ctx, ch)
	c.collectFirstAvailableBlock(ctx, ch)
	c.collectVoteAccounts(ctx, ch)

	// Collect version and firedancer status
	c.logger.Info("Collecting version...")
	version, err := c.rpcClient.GetVersion(ctx)
	isFiredancer := "0"
	if err == nil {
		resp, ferr := c.rpcClient.GetFiredancerMetrics(ctx)
		if ferr == nil && resp.StatusCode == 200 {
			isFiredancer = "1"
			c.isFiredancer = true
		}
	}
	if err != nil {
		c.logger.Errorf("failed to get version: %v", err)
		ch <- c.NodeVersion.NewInvalidMetric(err)
	} else {
		ch <- c.NodeVersion.MustNewConstMetric(1, version, isFiredancer)
	}
	c.logger.Info("Version collected.")

	c.collectIdentity(ctx, ch)
	c.collectBalances(ctx, ch)

	// Collect foundation min required version
	c.logger.Info("Collecting minimum required version...")
	genesisHash, err := c.rpcClient.GetGenesisHash(ctx)
	cluster := ""
	if err == nil {
		cluster, err = rpc.GetClusterFromGenesisHash(genesisHash)
	}
	agaveMinVersion, minVerCluster, epoch, firedancerMinVersion, minVerErr := "", "", 0, "", err
	if err == nil {
		agaveMinVersion, minVerCluster, epoch, firedancerMinVersion, minVerErr = c.apiClient.GetMinRequiredVersion(ctx, cluster)
	}
	if minVerErr != nil {
		c.logger.Errorf("failed to get min required version: %v", minVerErr)
		ch <- c.FoundationMinRequiredVersion.NewInvalidMetric(minVerErr)
	} else {
		ch <- c.FoundationMinRequiredVersion.MustNewConstMetric(1, agaveMinVersion, firedancerMinVersion, minVerCluster, fmt.Sprintf("%d", epoch))
	}
	c.logger.Info("Minimum required version collected.")

	// Collect NodeIsOutdated metric
	c.collectNodeIsOutdated(ch)

	// Collect NodeNeedsUpdate metric
	c.collectNodeNeedsUpdate(ch)

	c.logger.Info("=========== END COLLECTION ===========")
}
