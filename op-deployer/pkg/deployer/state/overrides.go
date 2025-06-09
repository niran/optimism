// File: op-deployer/pkg/deployer/state/valid_overrides.go

package state

import (
	"fmt"
	"sort"
	"strings"
)

// ValidOverrideKeys is a map of all keys that can be overridden in the deployment config
var ValidOverrideKeys = map[string]struct{}{
	// L2 Core parameters
	"l2BlockTime":               {},
	"l2GenesisBlockGasLimit":    {},
	"finalizationPeriodSeconds": {},
	"maxSequencerDrift":         {},
	"sequencerWindowSize":       {},

	// Fee vault parameters
	"l1FeeVaultMinimumWithdrawalAmount":        {},
	"sequencerFeeVaultMinimumWithdrawalAmount": {},
	"baseFeeVaultWithdrawalNetwork":            {},
	"l1FeeVaultWithdrawalNetwork":              {},
	"sequencerFeeVaultWithdrawalNetwork":       {},

	// Gas parameters
	"gasPriceOracleBaseFeeScalar":       {},
	"gasPriceOracleBlobBaseFeeScalar":   {},
	"gasPriceOracleOperatorFeeScalar":   {},
	"gasPriceOracleOperatorFeeConstant": {},

	// Fault proof parameters
	"useFaultProofs":                          {},
	"faultGameWithdrawalDelay":                {},
	"preimageOracleMinProposalSize":           {},
	"preimageOracleChallengePeriod":           {},
	"proofMaturityDelaySeconds":               {},
	"disputeGameFinalityDelaySeconds":         {},
	"mipsVersion":                             {},
	"respectedGameType":                       {},
	"faultGameAbsolutePrestate":               {},
	"faultGameMaxDepth":                       {},
	"faultGameSplitDepth":                     {},
	"faultGameClockExtension":                 {},
	"faultGameMaxClockDuration":               {},
	"dangerouslyAllowCustomDisputeParameters": {},

	// Hardfork timing parameters
	"l2GenesisRegolithTimeOffset":           {},
	"l2GenesisCanyonTimeOffset":             {},
	"l2GenesisDeltaTimeOffset":              {},
	"l2GenesisEcotoneTimeOffset":            {},
	"l2GenesisFjordTimeOffset":              {},
	"l2GenesisGraniteTimeOffset":            {},
	"l2GenesisHoloceneTimeOffset":           {},
	"l2GenesisIsthmusTimeOffset":            {},
	"l2GenesisInteropTimeOffset":            {},
	"l2GenesisJovianTimeOffset":             {},
	"l2GenesisPectraBlobScheduleTimeOffset": {},
}

// ValidateOverrides checks if all keys in the overrides map are valid override keys
func ValidateOverrides(overrides map[string]any) error {
	var invalidKeys []string

	for key := range overrides {
		if _, ok := ValidOverrideKeys[key]; !ok {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) > 0 {
		sort.Strings(invalidKeys)

		// Get valid keys for error message
		validKeysList := make([]string, 0, len(ValidOverrideKeys))
		for key := range ValidOverrideKeys {
			validKeysList = append(validKeysList, key)
		}
		sort.Strings(validKeysList)

		return fmt.Errorf("invalid override keys: %s\n\nValid keys are:\n%s\n",
			strings.Join(invalidKeys, ", "), strings.Join(validKeysList, "\n"))
	}

	return nil
}
