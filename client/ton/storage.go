package ton

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"

	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/keys"
	"github.com/xssnick/tonutils-go/adnl/overlay"
	"github.com/xssnick/tonutils-go/tl"
)

// tonutils-storage answers an overlay query for a bag it does not have with this text (storage/server.go, v1.5.5).
const storageBagNotFound = "not found"

type storageError struct {
	Message string `tl:"string"`
}

func init() {
	tl.Register(storageError{}, "storage.error message:string = storage.Error")
}

func checkStorageParams(params url.Values) error {
	if err := rejectParams(params, "bag"); err != nil {
		return err
	}
	if bag := params.Get("bag"); bag != "" {
		if _, err := decodeKey(bag); err != nil {
			return fmt.Errorf("bag: %w", err)
		}
	}
	return nil
}

// queryStorage asks for peers of a bag's overlay. tonutils-storage hands queries for overlays it does not know to
// the storage itself, so without ?bag a random overlay proves the storage is up: it answers "not found", and
// "storage is not yet initialized" while it is still starting.
func queryStorage(ctx context.Context, peer adnl.Peer, t *Target, r *Result) error {
	var overlayID []byte
	bag := t.Params.Get("bag")
	if bag == "" {
		overlayID = make([]byte, 32)
		if _, err := rand.Read(overlayID); err != nil {
			return err
		}
	} else {
		bagID, _ := hex.DecodeString(bag)
		var err error
		if overlayID, err = tl.Hash(keys.PublicKeyOverlay{Key: bagID}); err != nil {
			return err
		}
	}
	var answer tl.Serializable
	if err := peer.Query(ctx, overlay.WrapQuery(overlayID, overlay.GetRandomPeers{}), &answer); err != nil {
		return err
	}
	switch a := answer.(type) {
	case overlay.NodesList:
		return nil
	case storageError:
		if bag == "" && a.Message == storageBagNotFound {
			return nil
		}
		return errors.New(a.Message)
	default:
		return fmt.Errorf("unexpected answer %T", answer)
	}
}
