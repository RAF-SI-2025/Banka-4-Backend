package config

import (
	"errors"
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Peer struct {
	RoutingNumber int    `yaml:"routingNumber"`
	BaseURL       string `yaml:"baseUrl"`
	OurAPIKey     string `yaml:"ourApiKey"`
	TheirAPIKey   string `yaml:"theirApiKey"`
	DisplayName   string `yaml:"displayName"`
}

type PeersConfig struct {
	Peers []Peer `yaml:"peers"`
}

// PeerRegistry resolves peer banks by routing number or by the API key they
// present in inbound requests.
type PeerRegistry struct {
	byRouting  map[int]Peer
	byTheirKey map[string]Peer
}

func LoadPeers(path string) (*PeerRegistry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		// A missing peers file is a valid single-bank deployment: log it and
		// start with an empty registry so inter-bank features are simply
		// disabled rather than crashing the service. Other read errors
		// (permissions, etc.) are still fatal.
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("peers config %q not found; starting with no peers (inter-bank disabled)", path)
			return emptyRegistry(), nil
		}
		return nil, fmt.Errorf("read peers config %q: %w", path, err)
	}

	var cfg PeersConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse peers config: %w", err)
	}

	reg := emptyRegistry()

	for _, p := range cfg.Peers {
		if p.RoutingNumber == 0 || p.BaseURL == "" || p.OurAPIKey == "" || p.TheirAPIKey == "" {
			return nil, fmt.Errorf("peer %d: routingNumber, baseUrl, ourApiKey and theirApiKey are required", p.RoutingNumber)
		}

		if _, dup := reg.byRouting[p.RoutingNumber]; dup {
			return nil, fmt.Errorf("duplicate routingNumber %d", p.RoutingNumber)
		}

		if _, dup := reg.byTheirKey[p.TheirAPIKey]; dup {
			return nil, fmt.Errorf("duplicate theirApiKey for routingNumber %d", p.RoutingNumber)
		}

		reg.byRouting[p.RoutingNumber] = p
		reg.byTheirKey[p.TheirAPIKey] = p
	}
	return reg, nil
}

func emptyRegistry() *PeerRegistry {
	return &PeerRegistry{
		byRouting:  make(map[int]Peer),
		byTheirKey: make(map[string]Peer),
	}
}

// NewPeerRegistry constructs a registry from an explicit list. Useful for tests.
func NewPeerRegistry(peers []Peer) *PeerRegistry {
	reg := &PeerRegistry{
		byRouting:  make(map[int]Peer, len(peers)),
		byTheirKey: make(map[string]Peer, len(peers)),
	}
	for _, p := range peers {
		reg.byRouting[p.RoutingNumber] = p
		reg.byTheirKey[p.TheirAPIKey] = p
	}
	return reg
}

func (r *PeerRegistry) ByRoutingNumber(rn int) (Peer, bool) {
	p, ok := r.byRouting[rn]
	return p, ok
}

func (r *PeerRegistry) ByTheirAPIKey(key string) (Peer, bool) {
	p, ok := r.byTheirKey[key]
	return p, ok
}

func (r *PeerRegistry) All() []Peer {
	out := make([]Peer, 0, len(r.byRouting))
	for _, p := range r.byRouting {
		out = append(out, p)
	}
	return out
}
