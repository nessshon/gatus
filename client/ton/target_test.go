package ton

import (
	"strings"
	"testing"
)

const (
	testKey = "50d858e0985ecc7f60418aaf0cc5ab587f42c2570a884095a9e8ccacd0f6545c"
	testPub = "3b6a27bcceb6a42d62a3a8d02a6f0d73653215771de243a63ac048a18b59da29"
)

func TestParseTarget(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		name        string
		url         string
		wantService string
		wantAddress string
		wantErr     string
	}{
		{name: "adnl", url: "ton-adnl://" + testKey, wantService: "adnl"},
		{name: "storage", url: "ton-storage://" + testKey, wantService: "storage"},
		{name: "storage-with-bag", url: "ton-storage://" + testKey + "?bag=" + testPub, wantService: "storage"},
		{name: "provider", url: "ton-provider://" + testPub, wantService: "provider"},
		{name: "uppercase-key", url: "ton-storage://" + strings.ToUpper(testKey), wantService: "storage"},
		{name: "direct-address", url: "ton-storage://" + testPub + "@203.0.113.7:17555", wantService: "storage", wantAddress: "203.0.113.7:17555"},
		{name: "direct-ipv6", url: "ton-adnl://" + testPub + "@[2001:db8::1]:17555", wantService: "adnl", wantAddress: "[2001:db8::1]:17555"},
		{name: "unknown-service", url: "ton-liteserver://" + testKey, wantErr: "unknown TON service"},
		{name: "not-ton", url: "tcp://" + testKey, wantErr: "unknown TON service"},
		{name: "short-key", url: "ton-storage://abcd", wantErr: errInvalidKey.Error()},
		{name: "non-hex-key", url: "ton-storage://" + strings.Repeat("z", 64), wantErr: errInvalidKey.Error()},
		{name: "path", url: "ton-storage://" + testKey + "/bags", wantErr: "unexpected path"},
		{name: "hostname-address", url: "ton-storage://" + testPub + "@example.com:17555", wantErr: "address must be <ip>:<port>"},
		{name: "address-without-port", url: "ton-storage://" + testPub + "@203.0.113.7", wantErr: "address must be <ip>:<port>"},
		{name: "bad-public-key", url: "ton-storage://abcd@203.0.113.7:17555", wantErr: "public key"},
		{name: "public-key-off-curve", url: "ton-storage://" + strings.Repeat("0", 62) + "ff@203.0.113.7:17555", wantErr: "public key"},
		{name: "invalid-bag", url: "ton-storage://" + testKey + "?bag=abcd", wantErr: "bag"},
		{name: "unknown-storage-param", url: "ton-storage://" + testKey + "?size=1", wantErr: `unknown parameter "size"`},
		{name: "param-on-adnl", url: "ton-adnl://" + testKey + "?bag=" + testPub, wantErr: `unknown parameter "bag"`},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			target, err := ParseTarget(scenario.url)
			if scenario.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), scenario.wantErr) {
					t.Fatalf("expected error containing %q, got %v", scenario.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if target.Service != scenario.wantService {
				t.Errorf("expected service %q, got %q", scenario.wantService, target.Service)
			}
			if target.Address != scenario.wantAddress {
				t.Errorf("expected address %q, got %q", scenario.wantAddress, target.Address)
			}
			if scenario.wantAddress == "" && len(target.Key) != 32 {
				t.Errorf("expected a 32-byte key, got %d bytes", len(target.Key))
			}
			if scenario.wantAddress != "" && len(target.PublicKey) != 32 {
				t.Errorf("expected a 32-byte public key, got %d bytes", len(target.PublicKey))
			}
		})
	}
}
