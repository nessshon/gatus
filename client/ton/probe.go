package ton

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/address"
	"github.com/xssnick/tonutils-go/adnl/dht"
)

type State string

const (
	StateClientError  State = "client_error"
	StateDHTTimeout   State = "dht_timeout"
	StateDHTNotFound  State = "dht_not_found"
	StateNoAddress    State = "no_address"
	StateNoPing       State = "no_ping"
	StateNoService    State = "no_service"
	StateServiceError State = "service_error"
	StateOK           State = "ok"
)

// Upper bounds for a single round trip. Measured answers took 0.1-0.2s; the rest is headroom for lossy links.
const (
	pingTimeout  = 3 * time.Second
	queryTimeout = 3 * time.Second
)

var errNoAddress = errors.New("no dialable address in the DHT record")

type service struct {
	// checkParams validates the URL query. A service without it takes no parameters.
	checkParams func(url.Values) error

	lookup func(ctx context.Context, d *dht.Client, key []byte) (peerAddress, error)

	// query asks the service itself. A service without it is checked by the ADNL ping alone.
	query func(ctx context.Context, peer adnl.Peer, t *Target, r *Result) error
}

var services = map[string]service{
	"adnl":     {lookup: findAddress},
	"storage":  {checkParams: checkStorageParams, lookup: findAddress, query: queryStorage},
	"provider": {lookup: findProvider, query: queryProvider},
}

// Result is the outcome of a probe. It is serialized as the endpoint's body.
type Result struct {
	State   State  `json:"state"`
	Network string `json:"network"`
	Address string `json:"address,omitempty"`

	// DHT is "cache" or "lookup" when the address came from the DHT, empty when the URL named it.
	DHT         string `json:"dht,omitempty"`
	DHTAge      int64  `json:"dht_age_s,omitempty"`
	DHTTime     int64  `json:"dht_ms,omitempty"`
	PingTime    int64  `json:"ping_ms,omitempty"`
	ServiceTime int64  `json:"service_ms,omitempty"`
	Error       string `json:"error,omitempty"`

	Available *bool  `json:"available,omitempty"`
	MinSpan   uint32 `json:"min_span,omitempty"`
	MaxSpan   uint32 `json:"max_span,omitempty"`

	duration time.Duration
}

// Connected reports whether the peer answered the ADNL ping.
func (r *Result) Connected() bool {
	return r.State == StateNoService || r.State == StateServiceError || r.State == StateOK
}

// Duration is the service round trip, the ping for ton-adnl, and the whole probe when it failed.
func (r *Result) Duration() time.Duration {
	return r.duration
}

// Probe checks the service named by rawURL within timeout.
func Probe(rawURL string, n *Network, timeout time.Duration) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	r := &Result{}
	if n == nil {
		r.State, r.Error = StateClientError, "TON network is not configured"
	} else {
		r.Network = n.Name()
		var err error
		if r.State, err = r.run(ctx, rawURL, n); err != nil {
			r.Error = err.Error()
		}
	}
	if r.State != StateOK {
		r.duration = time.Since(start)
	}
	return r
}

func (r *Result) run(ctx context.Context, rawURL string, n *Network) (State, error) {
	t, err := ParseTarget(rawURL)
	if err != nil {
		return StateClientError, err
	}
	svc := services[t.Service]
	key := t.Service + ":" + hex.EncodeToString(t.Key)
	find := func(ctx context.Context) (peerAddress, error) {
		d, err := n.dhtClient(ctx)
		if err != nil {
			return peerAddress{}, &stateError{StateClientError, err}
		}
		addr, err := svc.lookup(ctx, d, t.Key)
		if errors.Is(err, dht.ErrDHTValueIsNotFound) && d.ActiveNodesCount() == 0 {
			return peerAddress{}, &stateError{StateClientError, errors.New("no DHT node answers")}
		}
		return addr, err
	}
	addr := peerAddress{addr: t.Address, pub: t.PublicKey}
	if t.Address == "" {
		if addr, err = r.resolve(ctx, n, key, find); err != nil {
			return lookupState(err), err
		}
	}
	gateway, peer, err := r.dial(ctx, addr)
	if err != nil && r.DHT == "cache" && dialState(err) != StateClientError {
		n.forget(key)
		fresh, lookupErr := r.resolve(ctx, n, key, find)
		if lookupErr != nil {
			return lookupState(lookupErr), lookupErr
		}
		if fresh.addr != addr.addr || !bytes.Equal(fresh.pub, addr.pub) {
			gateway, peer, err = r.dial(ctx, fresh)
		}
	}
	if err != nil {
		return dialState(err), err
	}
	defer gateway.Close()
	if svc.query == nil {
		return StateOK, nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	start := time.Now()
	err = svc.query(queryCtx, peer, t, r)
	r.duration = time.Since(start)
	r.ServiceTime = r.duration.Milliseconds()
	if errors.Is(err, context.DeadlineExceeded) {
		return StateNoService, err
	}
	if err != nil {
		return StateServiceError, err
	}
	return StateOK, nil
}

func (r *Result) resolve(ctx context.Context, n *Network, key string, find func(context.Context) (peerAddress, error)) (peerAddress, error) {
	start := time.Now()
	entry, cached, err := n.lookup(ctx, key, find)
	if err != nil {
		return peerAddress{}, fmt.Errorf("DHT lookup: %w", err)
	}
	if cached {
		r.DHT, r.DHTAge = "cache", int64(time.Since(entry.foundAt).Seconds())
	} else {
		r.DHT, r.DHTAge, r.DHTTime = "lookup", 0, time.Since(start).Milliseconds()
	}
	return entry.peerAddress, nil
}

// dial pings the peer from a gateway of its own. A peer closed on a shared gateway cannot be registered again
// right away, and a live one keeps sending into a stale channel for a while after the target restarts.
func (r *Result) dial(ctx context.Context, addr peerAddress) (*adnl.Gateway, adnl.Peer, error) {
	r.Address = addr.addr
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, &stateError{StateClientError, err}
	}
	gateway := adnl.NewGateway(key)
	if err = gateway.StartClient(); err != nil {
		return nil, nil, &stateError{StateClientError, err}
	}
	peer, err := gateway.RegisterClient(addr.addr, addr.pub)
	if err != nil {
		_ = gateway.Close()
		return nil, nil, &stateError{StateNoAddress, err}
	}
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	rtt, err := peer.Ping(pingCtx)
	if err != nil {
		_ = gateway.Close()
		return nil, nil, fmt.Errorf("no answer to ADNL ping from %s: %w", addr.addr, err)
	}
	r.PingTime, r.duration = rtt.Milliseconds(), rtt
	return gateway, peer, nil
}

// stateError fixes the state of a failure that the error itself does not tell.
type stateError struct {
	state State
	err   error
}

func (e *stateError) Error() string { return e.err.Error() }
func (e *stateError) Unwrap() error { return e.err }

func lookupState(err error) State {
	var se *stateError
	switch {
	case errors.As(err, &se):
		return se.state
	case errors.Is(err, errNoAddress):
		return StateNoAddress
	case errors.Is(err, dht.ErrDHTValueIsNotFound):
		return StateDHTNotFound
	case errors.Is(err, context.DeadlineExceeded):
		return StateDHTTimeout
	default:
		return StateClientError
	}
}

func dialState(err error) State {
	var se *stateError
	if errors.As(err, &se) {
		return se.state
	}
	return StateNoPing
}

func findAddress(ctx context.Context, d *dht.Client, id []byte) (peerAddress, error) {
	list, pub, err := d.FindAddresses(ctx, id)
	if err != nil {
		return peerAddress{}, err
	}
	for _, a := range list.Addresses {
		if s, err := address.DialString(a); err == nil {
			return peerAddress{addr: s, pub: pub}, nil
		}
	}
	return peerAddress{}, errNoAddress
}
