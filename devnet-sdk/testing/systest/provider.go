package systest

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/devnet-sdk/system"
)

// systemProvider defines the interface for package-level functionality
type systemProvider interface {
	NewSystemFromURL(string) (system.System, error)
	NewE2ESystem(BasicT) (system.System, error)
}

// defaultProvider is the default implementation of the package
type defaultProvider struct{}

var _ systemProvider = (*defaultProvider)(nil)

func (p *defaultProvider) NewSystemFromURL(url string) (system.System, error) {
	return system.NewSystemFromURL(url)
}

func (p *defaultProvider) NewE2ESystem(t BasicT) (system.System, error) {
	tt, ok := t.(*testing.T)
	if !ok {
		return nil, fmt.Errorf("e2e system test requires a *testing.T")
	}
	return system.NewE2ESystem(tt)
}
