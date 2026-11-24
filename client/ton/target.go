package ton

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/xssnick/tonutils-go/adnl/keys"
)

const schemePrefix = "ton-"

var errInvalidKey = errors.New("key must be 64 hex characters")

// Target is the service and peer named by a ton-<service>:// URL.
type Target struct {
	Service string

	// Key is what the service is found by in the DHT: the ADNL id, or the provider key for ton-provider.
	Key []byte

	// PublicKey and Address are set by the <pubkey>@<ip>:<port> form, which skips the DHT.
	PublicKey ed25519.PublicKey
	Address   string

	Params url.Values
}

// ParseTarget parses ton-<service>://<key>[?params] and ton-<service>://<pubkey>@<ip>:<port>[?params].
func ParseTarget(rawURL string) (*Target, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	name, hasPrefix := strings.CutPrefix(u.Scheme, schemePrefix)
	svc, known := services[name]
	if !hasPrefix || !known {
		return nil, fmt.Errorf("unknown TON service %q", u.Scheme)
	}
	if u.Path != "" {
		return nil, fmt.Errorf("unexpected path %q in TON URL", u.Path)
	}
	t := &Target{Service: name, Params: u.Query()}
	if u.User != nil {
		if t.PublicKey, err = decodeKey(u.User.Username()); err != nil {
			return nil, fmt.Errorf("public key: %w", err)
		}
		if _, err = keys.Ed25519PubToX25519(t.PublicKey); err != nil {
			return nil, fmt.Errorf("public key: %w", err)
		}
		if _, err = netip.ParseAddrPort(u.Host); err != nil {
			return nil, fmt.Errorf("address must be <ip>:<port>: %w", err)
		}
		t.Address = u.Host
	} else if t.Key, err = decodeKey(u.Host); err != nil {
		return nil, err
	}
	if svc.checkParams != nil {
		err = svc.checkParams(t.Params)
	} else {
		err = rejectParams(t.Params)
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func decodeKey(s string) ([]byte, error) {
	key, err := hex.DecodeString(s)
	if err != nil || len(key) != 32 {
		return nil, errInvalidKey
	}
	return key, nil
}

func rejectParams(params url.Values, allowed ...string) error {
	for name := range params {
		if !slices.Contains(allowed, name) {
			return fmt.Errorf("unknown parameter %q in TON URL", name)
		}
	}
	return nil
}
