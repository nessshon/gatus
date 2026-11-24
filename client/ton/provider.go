package ton

import (
	"context"
	"fmt"

	"github.com/xssnick/tonutils-go/adnl"
	"github.com/xssnick/tonutils-go/adnl/dht"
	"github.com/xssnick/tonutils-go/adnl/keys"
	"github.com/xssnick/tonutils-go/adnl/rldp"
	"github.com/xssnick/tonutils-go/tl"
)

// ratesAnswerLimit is the answer size limit the provider's own client uses (pkg/transport/client.go, v0.4.4).
const ratesAnswerLimit = 8196

type providerRecord struct {
	ADNLID []byte `tl:"int256"`
}

type ratesRequest struct {
	Size uint64 `tl:"long"`
}

type ratesResponse struct {
	Available        bool   `tl:"bool"`
	RatePerMBDay     []byte `tl:"bytes"`
	MinBounty        []byte `tl:"bytes"`
	SpaceAvailableMB uint64 `tl:"long"`
	MinSpan          uint32 `tl:"int"`
	MaxSpan          uint32 `tl:"int"`
}

// The schemas are copied from tonutils-storage-provider v0.4.4 (pkg/transport/types.go) as is: a TL id is the CRC
// of the schema text, so it has to match byte for byte, even where the text disagrees with the fields.
func init() {
	tl.Register(providerRecord{}, "storageProvider.dhtRecord adnl_key:int256 = storageProvider.DHTRecord")
	tl.Register(ratesRequest{}, "storageProvider.ratesRequest size:long = storageProvider.RatesRequest")
	tl.Register(ratesResponse{}, "storageProvider.ratesResponse available:Bool key:int256 rate_per_mb_day:bytes "+
		"reward_address:int256 space_available_mb:long min_span:int max_span:int = storageProvider.RatesResponse")
}

// findProvider follows the provider's own DHT record, signed by the provider key, to its ADNL server.
func findProvider(ctx context.Context, d *dht.Client, providerKey []byte) (peerAddress, error) {
	keyID, err := tl.Hash(keys.PublicKeyED25519{Key: providerKey})
	if err != nil {
		return peerAddress{}, err
	}
	value, _, err := d.FindValue(ctx, &dht.Key{ID: keyID, Name: []byte("storage-provider"), Index: 0})
	if err != nil {
		return peerAddress{}, err
	}
	var record providerRecord
	if _, err = tl.Parse(&record, value.Data, true); err != nil {
		return peerAddress{}, fmt.Errorf("%w: %v", errNoAddress, err)
	}
	return findAddress(ctx, d, record.ADNLID)
}

// queryProvider asks for storage rates over RLDP. The provider reports min(free space, asked size) as free space and
// is available only when the asked size fits (internal/service/service.go, v0.4.4), so asking for one byte turns
// "available" into "has room for new bags" and makes the space figure meaningless.
func queryProvider(ctx context.Context, peer adnl.Peer, _ *Target, r *Result) error {
	rldpClient := rldp.NewClientV2(peer)
	defer rldpClient.Close()
	var rates ratesResponse
	if err := rldpClient.DoQuery(ctx, ratesAnswerLimit, ratesRequest{Size: 1}, &rates); err != nil {
		return err
	}
	r.Available, r.MinSpan, r.MaxSpan = &rates.Available, rates.MinSpan, rates.MaxSpan
	return nil
}
