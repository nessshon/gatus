package ton

import (
	"errors"

	tonclient "github.com/TwiN/gatus/v5/client/ton"
)

// ErrUnknownNetwork is the error with which Gatus will panic if a TON endpoint is configured with an unknown network
var ErrUnknownNetwork = errors.New("ton.network must be mainnet or testnet")

// Config for an Endpoint of type TON
type Config struct {
	// Network is the TON network of the endpoint, mainnet by default
	Network string `yaml:"network,omitempty"`

	// ResolvedNetwork is the shared client of Network, set when the top-level ton configuration is validated
	ResolvedNetwork *tonclient.Network `yaml:"-"`
}

func (c *Config) ValidateAndSetDefaults() error {
	if c.Network == "" {
		c.Network = tonclient.NetworkMainnet
	}
	if _, ok := tonclient.Networks[c.Network]; !ok {
		return ErrUnknownNetwork
	}
	return nil
}
