package bindings

type ETHLockBoxFactory struct {
	BaseCallFactory
}

func NewETHLockBoxFactory(opts ...CallFactoryOption) *ETHLockBoxFactory {
	return &ETHLockBoxFactory{BaseCallFactory: *NewBaseCallFactory(opts...)}
}

type ETHLockbox struct {
	ETHLockBoxFactory

	// EmitLog func(topics []eth.Bytes32, data []byte) TypedCall[any] `sol:"emitLog"`
}

func NewETHLockBox(f *ETHLockBoxFactory) *ETHLockbox {
	c := ETHLockbox{ETHLockBoxFactory: *f}
	InitImpl(&c)
	return &c
}
