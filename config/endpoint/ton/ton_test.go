package ton

import (
	"testing"

	tonclient "github.com/TwiN/gatus/v5/client/ton"
)

func TestConfig_ValidateAndSetDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		t.Fatal(err)
	}
	if cfg.Network != tonclient.NetworkMainnet {
		t.Errorf("expected the default network to be %s, got %s", tonclient.NetworkMainnet, cfg.Network)
	}
	if err := (&Config{Network: tonclient.NetworkTestnet}).ValidateAndSetDefaults(); err != nil {
		t.Errorf("testnet should be valid, got %v", err)
	}
	if err := (&Config{Network: "tesnet"}).ValidateAndSetDefaults(); err != ErrUnknownNetwork {
		t.Errorf("expected %v, got %v", ErrUnknownNetwork, err)
	}
}
