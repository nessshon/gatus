package ton

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/keys"
	"github.com/xssnick/tonutils-go/adnl/overlay"
	"github.com/xssnick/tonutils-go/tl"
)

type fakePeer struct {
	adnl.Peer
	answer tl.Serializable
	err    error
	asked  tl.Serializable
}

func (p *fakePeer) Query(_ context.Context, req, result tl.Serializable) error {
	p.asked = req
	if p.err != nil {
		return p.err
	}
	*result.(*tl.Serializable) = p.answer
	return nil
}

func TestQueryStorage(t *testing.T) {
	t.Parallel()
	scenarios := []struct {
		name    string
		bag     string
		answer  tl.Serializable
		err     error
		wantErr string
	}{
		{name: "running storage without bag", answer: storageError{Message: storageBagNotFound}},
		{name: "storage still starting", answer: storageError{Message: "storage is not yet initialized"}, wantErr: "storage is not yet initialized"},
		{name: "active bag", bag: testPub, answer: overlay.NodesList{}},
		{name: "missing bag", bag: testPub, answer: storageError{Message: storageBagNotFound}, wantErr: storageBagNotFound},
		{name: "paused bag", bag: testPub, answer: storageError{Message: "bag " + testPub + " is not active"}, wantErr: "is not active"},
		{name: "unexpected answer", answer: overlay.Pong{}, wantErr: "unexpected answer"},
		{name: "no answer", err: context.DeadlineExceeded, wantErr: context.DeadlineExceeded.Error()},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			url := "ton-storage://" + testKey
			if scenario.bag != "" {
				url += "?bag=" + scenario.bag
			}
			target, err := ParseTarget(url)
			if err != nil {
				t.Fatal(err)
			}
			peer := &fakePeer{answer: scenario.answer, err: scenario.err}
			err = queryStorage(context.Background(), peer, target, &Result{})
			if scenario.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if scenario.wantErr != "" && (err == nil || !strings.Contains(err.Error(), scenario.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", scenario.wantErr, err)
			}
			if scenario.err != nil && !errors.Is(err, scenario.err) {
				t.Errorf("expected the query error to be kept, got %v", err)
			}
		})
	}
}

func TestQueryStorage_AsksTheBagOverlay(t *testing.T) {
	t.Parallel()
	target, err := ParseTarget("ton-storage://" + testKey + "?bag=" + testPub)
	if err != nil {
		t.Fatal(err)
	}
	peer := &fakePeer{answer: overlay.NodesList{}}
	if err = queryStorage(context.Background(), peer, target, &Result{}); err != nil {
		t.Fatal(err)
	}
	bagID, _ := hex.DecodeString(testPub)
	want, _ := tl.Hash(keys.PublicKeyOverlay{Key: bagID})
	query, ok := peer.asked.([]tl.Serializable)
	if !ok || len(query) != 2 {
		t.Fatalf("expected an overlay query, got %T", peer.asked)
	}
	if prefix, ok := query[0].(overlay.Query); !ok || !bytes.Equal(prefix.Overlay, want) {
		t.Errorf("expected the overlay of the bag, got %v", query[0])
	}
	if _, ok := query[1].(overlay.GetRandomPeers); !ok {
		t.Errorf("expected overlay.getRandomPeers, got %T", query[1])
	}
}
