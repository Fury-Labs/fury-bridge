package broadcast

import (
	"context"

	"github.com/fury-labs/fury-bridge/relayer/broadcast/types"
	"github.com/libp2p/go-libp2p-core/peer"
)

// Broadcaster defines the interface that a reliable broadcaster must implement.
type Broadcaster interface {
	BroadcastMessage(
		ctx context.Context,
		pb types.PeerMessage,
		recipients []peer.ID,
		TTLSeconds uint64,
	) error
}
