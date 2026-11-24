package ton

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testGlobalConfig = `{"@type":"config.global","dht":{"@type":"dht.config.global","k":6,"a":3,"static_nodes":{"@type":"dht.nodes","nodes":[]}},"liteservers":[]}`

func TestNetwork_lookupCachesAddress(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, "")
	var calls atomic.Int32
	find := func(context.Context) (peerAddress, error) {
		calls.Add(1)
		return peerAddress{addr: "203.0.113.7:17555"}, nil
	}
	first, cached, err := n.lookup(context.Background(), "storage:a", find)
	if err != nil || cached || first.addr != "203.0.113.7:17555" {
		t.Fatalf("first lookup: addr=%q cached=%v err=%v", first.addr, cached, err)
	}
	second, cached, err := n.lookup(context.Background(), "storage:a", find)
	if err != nil || !cached || second.addr != first.addr {
		t.Fatalf("second lookup: addr=%q cached=%v err=%v", second.addr, cached, err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected 1 search, got %d", calls.Load())
	}
	n.forget("storage:a")
	if _, cached, _ = n.lookup(context.Background(), "storage:a", find); cached {
		t.Error("expected a new search after forget")
	}
}

func TestNetwork_lookupExpiresAddress(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, "")
	n.cache["storage:a"] = cachedAddress{peerAddress: peerAddress{addr: "old:1"}, foundAt: time.Now().Add(-addressTTL)}
	entry, cached, err := n.lookup(context.Background(), "storage:a", func(context.Context) (peerAddress, error) {
		return peerAddress{addr: "new:1"}, nil
	})
	if err != nil || cached || entry.addr != "new:1" {
		t.Fatalf("expected a new search, got addr=%q cached=%v err=%v", entry.addr, cached, err)
	}
}

func TestNetwork_lookupDoesNotCacheErrors(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, "")
	errLookup := errors.New("lookup failed")
	if _, _, err := n.lookup(context.Background(), "storage:a", func(context.Context) (peerAddress, error) {
		return peerAddress{}, errLookup
	}); !errors.Is(err, errLookup) {
		t.Fatalf("expected the lookup error, got %v", err)
	}
	if len(n.cache) != 0 {
		t.Error("a failed lookup must not be cached")
	}
}

func TestNetwork_lookupMergesConcurrentSearches(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, "")
	var calls atomic.Int32
	release := make(chan struct{})
	find := func(context.Context) (peerAddress, error) {
		calls.Add(1)
		<-release
		return peerAddress{addr: "203.0.113.7:17555"}, nil
	}
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = n.lookup(context.Background(), "storage:a", find)
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Errorf("expected 1 search, got %d", calls.Load())
	}
}

func TestLoadGlobalConfig(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/global.config.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(testGlobalConfig))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "global.config.json")
	if err := os.WriteFile(path, []byte(testGlobalConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{server.URL + "/global.config.json", path} {
		cfg, err := loadGlobalConfig(context.Background(), source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		if cfg.DHT.K != 6 {
			t.Errorf("%s: expected k=6, got %d", source, cfg.DHT.K)
		}
	}
	if _, err := loadGlobalConfig(context.Background(), server.URL+"/missing.json"); err == nil {
		t.Error("expected an error for a 404 response")
	}
	if _, err := loadGlobalConfig(context.Background(), filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestNetwork_dhtClientAfterClose(t *testing.T) {
	t.Parallel()
	n := NewNetwork(NetworkMainnet, "unused")
	n.Close()
	if _, err := n.dhtClient(context.Background()); !errors.Is(err, errNetworkClosed) {
		t.Errorf("expected %v, got %v", errNetworkClosed, err)
	}
}
