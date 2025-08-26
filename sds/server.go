package sds

import (
	"fmt"
	"log/slog"
	"time"

	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secret "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type SecretsManager interface {
	Register(sessionID string, resourceNames []string) <-chan []auth.Secret
	Unregister(sessionID string) error
}

type Server struct {
	secret.UnimplementedSecretDiscoveryServiceServer

	Logger *slog.Logger
	Stop   chan struct{}

	Manager SecretsManager
}

func (s *Server) StreamSecrets(stream secret.SecretDiscoveryService_StreamSecretsServer) error {
	logger := s.Logger
	logger.Info("StreamSecrets called")

	errch := make(chan error)
	reqs := make(chan *discovery.DiscoveryRequest)

	// Redirect messages from the client to channel
	go func() {
		defer close(errch)
		defer close(reqs)

		for {
			r, err := stream.Recv()
			if err != nil {
				errch <- err
				return
			}

			select {
			case reqs <- r:
			case <-stream.Context().Done():
				return
			case <-s.Stop:
				return
			}
		}
	}()

	nodeID, sessionID, secretsch, err := s.processInitialRequest(reqs, errch)
	if err != nil {
		logger.Info("failed to process first request on the stream", slog.Any("error", err))
		return err
	}

	logger = logger.With(
		slog.String("node_id", nodeID),
		slog.String("session_id", sessionID),
	)
	defer func() {
		if err := s.Manager.Unregister(sessionID); err != nil {
			logger.Error("failed to unregister session", slog.Any("error", err))
		}
	}()

	var nonce, version string
	for {
		select {
		case r, ok := <-reqs:
			if !ok {
				// prevent CPU spin loop.
				return nil
			}

			switch {
			case r == nil:
				continue
			case r.ErrorDetail != nil:
				logger.Error("node returned NACK", slog.String("error", r.ErrorDetail.Message))
			case r.ResponseNonce != nonce:
				logger.Info("received invalid nonce")
			case r.VersionInfo == version:
				logger.Info("node returned ACK")
			default:
				logger.Info("unexpected request case")
			}
		case secrets := <-secretsch:
			logger.Info("updating secrets")

			nonce = uuid.NewString()
			version = versionInfo()
			resp, err := createDiscoveryResponse(nonce, version, secrets)
			if err != nil {
				logger.Error("failed to create DiscoveryResponse", slog.Any("error", err))
				return err
			}

			if err := stream.Send(resp); err != nil {
				logger.Error("failed to send DiscoveryResponse to client", slog.Any("error", err))
				return err
			}
		case err := <-errch:
			if err == nil {
				logger.Info("error channel was closed")
				return nil
			}
			logger.Error("StreamSecrets failed to receive message", slog.Any("error", err))
			return err
		case <-s.Stop:
			logger.Info("SDS server is stopped")
			return nil
		}
	}
}

// processInitialRequest processes first request received from the client on the stream.
// Arguments:
//   - reqs: channel for requests
//   - errch: channel for errors that occur durin receiving/processing requests
func (s *Server) processInitialRequest(
	reqs <-chan *discovery.DiscoveryRequest,
	errch <-chan error,
) (nodeID, sessionID string, secretsch <-chan []auth.Secret, err error) {
	for {
		select {
		case r, ok := <-reqs:
			if !ok {
				// prevent CPU spin loop.
				return "", "", nil, fmt.Errorf("request channel was closed before receiving request")
			}

			nodeID = r.Node.Id
			sessionID = uuid.NewString()
			secretsch = s.Manager.Register(sessionID, r.ResourceNames)
			return nodeID, sessionID, secretsch, nil
		case err, ok := <-errch:
			if !ok {
				return "", "", nil, fmt.Errorf("error channel was closed before receiving request")
			}

			return "", "", nil, fmt.Errorf("failed to receive message from the stream: %w", err)
		case <-s.Stop:
			return "", "", nil, fmt.Errorf("SDS server is stopped before receiving request")
		}
	}
}

func versionInfo() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func createDiscoveryResponse(nonce, version string, secrets []auth.Secret) (*discovery.DiscoveryResponse, error) {
	resources := make([]*anypb.Any, 0, len(secrets))
	for _, s := range secrets {
		b, err := proto.Marshal(&s)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal secret: %s: %w", s.Name, err)
		}

		resources = append(resources, &anypb.Any{
			TypeUrl: resource.SecretType,
			Value:   b,
		})
	}

	return &discovery.DiscoveryResponse{
		VersionInfo: version,
		Resources:   resources,
		TypeUrl:     resource.SecretType,
		Nonce:       nonce,
	}, nil
}
