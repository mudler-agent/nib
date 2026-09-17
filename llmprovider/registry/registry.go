// Package registry maps wire protocols to adapter factories. Each adapter
// package (anthropic, google, …) calls Register in an init() so that
// llmprovider.New can discover it without a manual switch-case.
//
// Adding a new native adapter is three steps:
//  1. Write the adapter package (implement cogito.LLM).
//  2. Add an init() that calls registry.Register with the protocol constant.
//  3. Blank-import the package in llmprovider/provider.go.
//
// No changes to provider.go's dispatch logic are needed.
package registry

import (
	"github.com/mudler/cogito"
	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

// Factory builds a cogito.LLM for one provider definition. temperature is
// passed through for OpenAI-compatible providers that use it; native adapters
// (anthropic, google) ignore it.
type Factory func(def provider.Definition, config types.ModelProviderConfig, store *auth.Store, temperature float32) (cogito.LLM, error)

var factories = map[provider.Protocol]Factory{}

// Register associates a wire protocol with its adapter factory. Panics on
// duplicate registration — this is a programming error caught at init time.
func Register(p provider.Protocol, f Factory) {
	if _, ok := factories[p]; ok {
		panic("registry: protocol " + string(p) + " already registered")
	}
	factories[p] = f
}

// Get returns the factory for a protocol, or (nil, false) if no adapter has
// registered for it.
func Get(p provider.Protocol) (Factory, bool) {
	f, ok := factories[p]
	return f, ok
}
