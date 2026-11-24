package ton

import (
	"fmt"

	tonclient "github.com/TwiN/gatus/v5/client/ton"
)

// Config is the shared configuration of TON endpoints
type Config struct {
	// GlobalConfigs maps a network to the URL or path of its global config. Networks left out use the official one.
	GlobalConfigs map[string]string `yaml:"config,omitempty"`

	networks map[string]*tonclient.Network
}

// ValidateAndSetDefaults validates the configuration and creates a client for each network
func (c *Config) ValidateAndSetDefaults() error {
	for name, source := range c.GlobalConfigs {
		if _, ok := tonclient.Networks[name]; !ok {
			return fmt.Errorf("ton.config: unknown network %q", name)
		}
		if source == "" {
			return fmt.Errorf("ton.config.%s: URL or path is empty", name)
		}
	}
	c.networks = make(map[string]*tonclient.Network, len(tonclient.Networks))
	for name, source := range tonclient.Networks {
		if custom, ok := c.GlobalConfigs[name]; ok {
			source = custom
		}
		c.networks[name] = tonclient.NewNetwork(name, source)
	}
	return nil
}

// Network returns the client of the network with the given name
func (c *Config) Network(name string) *tonclient.Network {
	return c.networks[name]
}

// Close stops the clients of all networks
func (c *Config) Close() {
	for _, network := range c.networks {
		network.Close()
	}
}
