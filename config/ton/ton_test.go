package ton

import (
	"testing"

	tonclient "github.com/TwiN/gatus/v5/client/ton"
)

func TestConfig_ValidateAndSetDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{GlobalConfigs: map[string]string{tonclient.NetworkTestnet: "/etc/gatus/testnet.json"}}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}
	for name := range tonclient.Networks {
		if network := cfg.Network(name); network == nil || network.Name() != name {
			t.Errorf("expected a client for %s", name)
		}
	}
	if cfg.Network("devnet") != nil {
		t.Error("expected no client for an unknown network")
	}
}

func TestConfig_ValidateAndSetDefaultsWithInvalidConfig(t *testing.T) {
	t.Parallel()
	for name, globalConfigs := range map[string]map[string]string{
		"unknown-network": {"devnet": "https://example.org/devnet.json"},
		"empty-source":    {tonclient.NetworkMainnet: ""},
	} {
		if err := (&Config{GlobalConfigs: globalConfigs}).ValidateAndSetDefaults(); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
