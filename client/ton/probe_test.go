package ton

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/xssnick/tonutils-go/adnl/dht"
)

func TestProbe_WithoutNetwork(t *testing.T) {
	t.Parallel()
	r := Probe("ton-storage://"+testKey, nil, time.Second)
	if r.State != StateClientError || r.Connected() {
		t.Errorf("expected a client error without connection, got state=%s connected=%v", r.State, r.Connected())
	}
}

func TestProbe_InvalidURL(t *testing.T) {
	t.Parallel()
	r := Probe("ton-storage://abcd", NewNetwork(NetworkMainnet, ""), time.Second)
	if r.State != StateClientError || r.Error == "" {
		t.Errorf("expected a client error with a message, got state=%s error=%q", r.State, r.Error)
	}
	if r.Network != NetworkMainnet {
		t.Errorf("expected network %q, got %q", NetworkMainnet, r.Network)
	}
}

func TestProbe_UnreadableGlobalConfig(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, filepath.Join(t.TempDir(), "missing.json"))
	r := Probe("ton-storage://"+testKey, n, time.Second)
	if r.State != StateClientError {
		t.Errorf("expected %s, got %s (%s)", StateClientError, r.State, r.Error)
	}
	if r.Duration() <= 0 {
		t.Error("a failed probe must report its duration")
	}
}

func TestLookupState(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		err  error
		want State
	}{
		{err: fmt.Errorf("DHT lookup: %w", dht.ErrDHTValueIsNotFound), want: StateDHTNotFound},
		{err: fmt.Errorf("DHT lookup: %w", context.DeadlineExceeded), want: StateDHTTimeout},
		{err: fmt.Errorf("DHT lookup: %w", errNoAddress), want: StateNoAddress},
		{err: fmt.Errorf("DHT lookup: %w", &stateError{StateClientError, errors.New("no DHT node answers")}), want: StateClientError},
		{err: errors.New("anything else"), want: StateClientError},
	}
	for _, scenario := range scenarios {
		if got := lookupState(scenario.err); got != scenario.want {
			t.Errorf("%v: expected %s, got %s", scenario.err, scenario.want, got)
		}
	}
}

func TestDialState(t *testing.T) {
	t.Parallel()
	if got := dialState(&stateError{StateClientError, errors.New("socket")}); got != StateClientError {
		t.Errorf("expected %s, got %s", StateClientError, got)
	}
	if got := dialState(&stateError{StateNoAddress, errors.New("bad address")}); got != StateNoAddress {
		t.Errorf("expected %s, got %s", StateNoAddress, got)
	}
	if got := dialState(fmt.Errorf("no answer to ADNL ping: %w", context.DeadlineExceeded)); got != StateNoPing {
		t.Errorf("expected %s, got %s", StateNoPing, got)
	}
}

func TestResult_Connected(t *testing.T) {
	t.Parallel()
	for state, want := range map[State]bool{
		StateClientError:  false,
		StateDHTTimeout:   false,
		StateDHTNotFound:  false,
		StateNoAddress:    false,
		StateNoPing:       false,
		StateNoService:    true,
		StateServiceError: true,
		StateOK:           true,
	} {
		if got := (&Result{State: state}).Connected(); got != want {
			t.Errorf("%s: expected connected=%v, got %v", state, want, got)
		}
	}
}
