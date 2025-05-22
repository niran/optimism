package processors

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type EventDecoderFn func(*ethTypes.Log, depset.ChainCodeFromID) (*types.ExecutingMessage, error)

func DecodeExecutingMessageLog(l *ethTypes.Log, depSet depset.ChainCodeFromID) (*types.ExecutingMessage, error) {
	if l.Address != params.InteropCrossL2InboxAddress {
		return nil, nil
	}
	if len(l.Topics) != 2 { // topics: event-id and payload-hash
		return nil, nil
	}
	if l.Topics[0] != types.ExecutingMessageEventTopic {
		return nil, nil
	}
	var msg types.Message
	if err := msg.DecodeEvent(l.Topics, l.Data); err != nil {
		return nil, fmt.Errorf("invalid executing message: %w", err)
	}
	logHash := types.PayloadHashToLogHash(msg.PayloadHash, msg.Identifier.Origin)

	var chainCode types.ChainCode
	index, err := depSet.ChainCodeFromID(eth.ChainID(msg.Identifier.ChainID))
	if err != nil {
		if errors.Is(err, types.ErrUnknownChain) {
			chainCode = depset.NotFoundChainCode
		} else {
			return nil, fmt.Errorf("failed to translate chain ID %s to chain index: %w", msg.Identifier.ChainID, err)
		}
	} else {
		chainCode = index
	}
	return &types.ExecutingMessage{
		Chain:     chainCode,
		BlockNum:  msg.Identifier.BlockNumber,
		LogIdx:    msg.Identifier.LogIndex,
		Timestamp: msg.Identifier.Timestamp,
		Hash:      logHash,
	}, nil
}
