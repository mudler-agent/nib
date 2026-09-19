package types

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Endpoint is one named model endpoint config.yaml offers besides its
// default top-level block. Addressing fields are self-contained; behavior
// fields inherit from the default when unset (see Config.ResolvedEndpoint).
type Endpoint struct {
	Name                string `yaml:"-"`
	ModelProviderConfig `yaml:",inline"`
}

// Endpoints is the `endpoints:` mapping, kept in file order. A Go map would
// reshuffle the picker, the boot log and the listing on every render, so the
// mapping is decoded through its node rather than into a map.
type Endpoints []Endpoint

// UnmarshalYAML decodes a mapping of name to endpoint, preserving key order.
func (e *Endpoints) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("endpoints must be a mapping of name to endpoint, got %s", nodeKind(value))
	}
	out := make(Endpoints, 0, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		var ep Endpoint
		if err := value.Content[i+1].Decode(&ep.ModelProviderConfig); err != nil {
			return fmt.Errorf("endpoint %q: %w", value.Content[i].Value, err)
		}
		ep.Name = value.Content[i].Value
		out = append(out, ep)
	}
	*e = out
	return nil
}

func nodeKind(n *yaml.Node) string {
	switch n.Kind {
	case yaml.SequenceNode:
		return "a list"
	case yaml.ScalarNode:
		return "a scalar"
	default:
		return "an unexpected node"
	}
}

// Validate returns the usable entries and one error per rejected entry, so a
// bad endpoint names itself in the boot log instead of vanishing.
//
// An entry must carry base_url or provider: `model` alone is rejected
// because addressing does not inherit, so such an entry would silently
// resolve to the OpenAI SDK's default host rather than to the default
// endpoint's base_url.
func (e Endpoints) Validate() (Endpoints, []error) {
	var (
		out  Endpoints
		errs []error
		seen = map[string]bool{}
	)
	for _, ep := range e {
		switch {
		case strings.TrimSpace(ep.Name) == "":
			errs = append(errs, fmt.Errorf("endpoint with an empty name was ignored"))
		case strings.ContainsAny(ep.Name, " \t"):
			errs = append(errs, fmt.Errorf("endpoint %q was ignored: the name cannot contain whitespace", ep.Name))
		case strings.Contains(ep.Name, "@"):
			errs = append(errs, fmt.Errorf("endpoint %q was ignored: the name cannot contain @", ep.Name))
		case seen[ep.Name]:
			errs = append(errs, fmt.Errorf("endpoint %q was ignored: duplicate name", ep.Name))
		case ep.BaseURL == "" && ep.Provider == "":
			errs = append(errs, fmt.Errorf("endpoint %q was ignored: it needs base_url or provider, model alone is not an address", ep.Name))
		default:
			seen[ep.Name] = true
			out = append(out, ep)
		}
	}
	return out, errs
}
