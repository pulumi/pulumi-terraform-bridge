package internalinter

type InternalInterface interface {
	noimplement()
}

type Internal struct{}

func (s Internal) noimplement() {}
