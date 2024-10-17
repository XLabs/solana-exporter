package rpc

// error codes: https://github.com/anza-xyz/agave/blob/489f483e1d7b30ef114e0123994818b2accfa389/rpc-client-api/src/custom_error.rs#L17
const (
	BlockCleanedUpCode                           = -32001
	SendTransactionPreflightFailureCode          = -32002
	TransactionSignatureVerificationFailureCode  = -32003
	BlockNotAvailableCode                        = -32004
	NodeUnhealthyCode                            = -32005
	TransactionPrecompileVerificationFailureCode = -32006
	SlotSkippedCode                              = -32007
	NoSnapshotCode                               = -32008
	LongTermStorageSlotSkippedCode               = -32009
	KeyExcludedFromSecondaryIndexCode            = -32010
	TransactionHistoryNotAvailableCode           = -32011
	ScanErrorCode                                = -32012
	TransactionSignatureLengthMismatchCode       = -32013
	BlockStatusNotYetAvailableCode               = -32014
	UnsupportedTransactionVersionCode            = -32015
	MinContextSlotNotReachedCode                 = -32016
	EpochRewardsPeriodActiveCode                 = -32017
	SlotNotEpochBoundaryCode                     = -32018
)