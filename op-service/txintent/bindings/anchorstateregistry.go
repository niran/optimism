package bindings

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type AnchorStateRegistryFactory struct {
	BaseCallFactory
}

func NewAnchorStateRegistryFactory(opts ...CallFactoryOption) *AnchorStateRegistryFactory {
	return &AnchorStateRegistryFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

type AnchorStateRegistry struct {
	AnchorStateRegistryFactory

	IsGameClaimValid func(common.Address) TypedCall[bool] `sol:"isGameClaimValid"`
	IsGameProper     func(common.Address) TypedCall[bool] `sol:"isGameProper"`
	IsGameRespected  func(common.Address) TypedCall[bool] `sol:"isGameRespected"`
	IsGameFinalized  func(common.Address) TypedCall[bool] `sol:"isGameFinalized"`
	IsGameResolved   func(common.Address) TypedCall[bool] `sol:"isGameResolved"`

	DisputeGameFinalityDelaySeconds func() TypedCall[*big.Int] `sol:"disputeGameFinalityDelaySeconds"`
	// GameData func() TypedCall[struct {
	// 	GameType  uint32
	// 	RootClaim [32]byte
	// 	ExtraData []byte
	// }] `sol:"gameData"`
}

func NewAnchorStateRegistry(f *AnchorStateRegistryFactory) *AnchorStateRegistry {
	c := AnchorStateRegistry{AnchorStateRegistryFactory: *f}
	InitImpl(&c)
	return &c
}
