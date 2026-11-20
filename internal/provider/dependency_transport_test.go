package provider

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// Terraform's plugin RPC transport uses x/net/http2 framing through gRPC.
// Exercise concurrent streams and trailers without sockets, API credentials,
// acceptance-test opt-in, or changes to any OpenAI organization.
func TestDependencyGRPCMetadata(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	t.Cleanup(func() { _ = listener.Close() })
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := grpc.SetHeader(ctx, metadata.Pairs("fixture-header", "synthetic")); err != nil {
			return nil, err
		}
		if err := grpc.SetTrailer(ctx, metadata.Pairs("fixture-trailer", "synthetic")); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}))
	healthpb.RegisterHealthServer(server, health.NewServer())
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-served; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("Server.Serve(in-memory listener) = %v, want nil or ErrServerStopped", err)
		}
	})
	conn, err := grpc.NewClient("passthrough:///offline-plugin-transport",
		// These credentials apply only to the in-memory test connection.
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
	)
	if err != nil {
		t.Fatalf("NewClient(in-memory listener) = %v, want nil error", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := healthpb.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	var calls sync.WaitGroup
	for range 32 {
		calls.Go(func() {
			var header, trailer metadata.MD
			response, err := client.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.Header(&header), grpc.Trailer(&trailer))
			if err != nil {
				t.Errorf("Health.Check(empty service) = %v, want nil error", err)
				return
			}
			if got := response.GetStatus(); got != healthpb.HealthCheckResponse_SERVING {
				t.Errorf("Health.Check(empty service).status = %v, want SERVING", got)
			}
			if got := header.Get("fixture-header"); !slices.Equal(got, []string{"synthetic"}) {
				t.Errorf("Health.Check(empty service).header = %v, want [synthetic]", got)
			}
			if got := trailer.Get("fixture-trailer"); !slices.Equal(got, []string{"synthetic"}) {
				t.Errorf("Health.Check(empty service).trailer = %v, want [synthetic]", got)
			}
		})
	}
	calls.Wait()
}
