package proxy

import (
	"context"
	"fmt"
	"github.com/siderolabs/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"net"
	"net/netip"
	"sync"
	"time"
)

// RemoteBackend is a proxy.One2ManyResponder implementation that proxies to a remote gRPC server, injecting machine metadata
// into the response.
//
// Based on the Talos apid implementation:
// https://github.com/siderolabs/talos/blob/59a78da42cdea8fbccc35d0851f9b0eef928261b/internal/app/apid/pkg/backend/apid.go
type RemoteBackend struct {
	One2ManyResponder
	target string

	mu   sync.RWMutex
	conn *grpc.ClientConn
}

var _ proxy.Backend = (*RemoteBackend)(nil)

// NewRemoteBackend creates a new instance of RemoteBackend for the given target which must have the format [IPv6]:port.
func NewRemoteBackend(target string) (*RemoteBackend, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("target must have the format [IPv6]:port: %s", target)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is6() {
		return nil, fmt.Errorf("target host must be a valid IPv6 address: %s", host)
	}

	return &RemoteBackend{
		One2ManyResponder: One2ManyResponder{
			machine: target,
		},
		target: target,
	}, nil
}

func (b *RemoteBackend) String() string {
	return b.machine
}

// GetConnection returns a gRPC connection to the remote server.
func (b *RemoteBackend) GetConnection(ctx context.Context, _ string) (context.Context, *grpc.ClientConn, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if authority := md[":authority"]; len(authority) > 0 {
		md.Set("proxy-authority", authority...)
	} else {
		md.Set("proxy-authority", "unknown")
	}
	delete(md, ":authority")
	delete(md, "machines")

	outCtx := metadata.NewOutgoingContext(ctx, md)

	b.mu.RLock()
	if b.conn != nil {
		defer b.mu.RUnlock()
		return outCtx, b.conn, nil
	}
	b.mu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Override the max delay to avoid excessive backoff when the another node is unavailable, e.g. rebooted
	// or the WireGuard connection is temporary down.
	//
	// Default max delay is 2 minutes, which is too long for our use case.
	backoffConfig := backoff.DefaultConfig
	// The maximum wait time between attempts.
	backoffConfig.MaxDelay = 15 * time.Second

	var err error
	b.conn, err = grpc.NewClient(
		b.target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoffConfig,
			// Not published as a constant in gRPC library.
			// See: https://github.com/grpc/grpc-go/blob/d5dee5fdbdeb52f6ea10b37b2cc7ce37814642d7/clientconn.go#L55-L56
			// Each connection attempt can take up to MinConnectTimeout.
			MinConnectTimeout: 20 * time.Second,
		}),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodecV2(proxy.Codec()),
		),
	)

	return outCtx, b.conn, err
}

// Close closes the upstream gRPC connection.
func (b *RemoteBackend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
}
