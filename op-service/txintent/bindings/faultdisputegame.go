package bindings

type FaultDisputeGameFactory struct {
	BaseCallFactory
}

func NewFaultDisputeGameFactory(opts ...CallFactoryOption) *FaultDisputeGameFactory {
	return &FaultDisputeGameFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

type FaultDisputeGame struct {
	FaultDisputeGameFactory

	GameData func() TypedCall[struct {
		GameType  uint32
		RootClaim [32]byte
		ExtraData []byte
	}] `sol:"gameData"`
}

func NewFaultDisputeGame(f *FaultDisputeGameFactory) *FaultDisputeGame {
	c := FaultDisputeGame{FaultDisputeGameFactory: *f}
	InitImpl(&c)
	return &c
}
