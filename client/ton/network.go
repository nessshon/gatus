package ton

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TwiN/gatus/v5/client"
	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/dht"
	"github.com/xssnick/tonutils-go/liteclient"
	"golang.org/x/sync/singleflight"
)

const (
	NetworkMainnet = "mainnet"
	NetworkTestnet = "testnet"
)

// Networks maps each supported network to its official global config.
var Networks = map[string]string{
	NetworkMainnet: "https://ton-blockchain.github.io/global.config.json",
	NetworkTestnet: "https://ton-blockchain.github.io/testnet-global.config.json",
}

var errNetworkClosed = errors.New("TON network client is closed")

// addressTTL bounds how long a found address is reused. It is shorter than the TTL that tonutils-storage (20 min)
// and tonutils-storage-provider (30 min) put on their DHT records, so a record that is gone shows up within it.
const addressTTL = 5 * time.Minute

type peerAddress struct {
	addr string
	pub  ed25519.PublicKey
}

type cachedAddress struct {
	peerAddress
	foundAt time.Time
}

// Network holds the DHT client and the address cache shared by all checks of one TON network.
// The client is started by the first check that needs it.
type Network struct {
	name   string
	source string

	mu    sync.Mutex
	dht   *dht.Client
	cache map[string]cachedAddress

	// closed keeps a check that is still running during a configuration reload from starting a client
	// that nothing would close.
	closed bool

	starts  singleflight.Group
	lookups singleflight.Group
}

// NewNetwork returns a network whose global config is read from source, a URL or a file path.
func NewNetwork(name, source string) *Network {
	return &Network{name: name, source: source, cache: make(map[string]cachedAddress)}
}

func (n *Network) Name() string {
	return n.name
}

// Close stops the DHT client and drops the cached addresses.
func (n *Network) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.dht != nil {
		n.dht.Close()
		n.dht = nil
	}
	clear(n.cache)
	n.closed = true
}

// dhtClient returns the DHT client, starting it on first use. Concurrent checks wait for one start instead of each
// downloading the global config.
func (n *Network) dhtClient(ctx context.Context) (*dht.Client, error) {
	n.mu.Lock()
	d, closed := n.dht, n.closed
	n.mu.Unlock()
	if closed {
		return nil, errNetworkClosed
	}
	if d != nil {
		return d, nil
	}
	v, err, _ := n.starts.Do("", func() (any, error) {
		return n.startDHT(ctx)
	})
	if err != nil {
		return nil, err
	}
	return v.(*dht.Client), nil
}

func (n *Network) startDHT(ctx context.Context) (*dht.Client, error) {
	cfg, err := loadGlobalConfig(ctx, n.source)
	if err != nil {
		return nil, fmt.Errorf("global config: %w", err)
	}
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	gateway := adnl.NewGateway(key)
	if err = gateway.StartClient(); err != nil {
		return nil, err
	}
	d, err := dht.NewClientFromConfig(gateway, cfg)
	if err != nil {
		_ = gateway.Close()
		return nil, err
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		d.Close()
		return nil, errNetworkClosed
	}
	if n.dht != nil {
		d.Close()
		return n.dht, nil
	}
	n.dht = d
	return d, nil
}

// lookup returns the cached address for key or finds it, merging concurrent searches for the same key.
func (n *Network) lookup(ctx context.Context, key string, find func(context.Context) (peerAddress, error)) (cachedAddress, bool, error) {
	n.mu.Lock()
	entry, ok := n.cache[key]
	n.mu.Unlock()
	if ok && time.Since(entry.foundAt) < addressTTL {
		return entry, true, nil
	}
	v, err, _ := n.lookups.Do(key, func() (any, error) {
		addr, err := find(ctx)
		if err != nil {
			return nil, err
		}
		found := cachedAddress{peerAddress: addr, foundAt: time.Now()}
		n.mu.Lock()
		n.cache[key] = found
		n.mu.Unlock()
		return found, nil
	})
	if err != nil {
		return cachedAddress{}, false, err
	}
	return v.(cachedAddress), false, nil
}

func (n *Network) forget(key string) {
	n.mu.Lock()
	delete(n.cache, key)
	n.mu.Unlock()
}

func loadGlobalConfig(ctx context.Context, source string) (*liteclient.GlobalConfig, error) {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		return liteclient.GetConfigFromFile(source)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.GetHTTPClient(nil).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", source, response.Status)
	}
	var cfg liteclient.GlobalConfig
	if err = json.NewDecoder(response.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
