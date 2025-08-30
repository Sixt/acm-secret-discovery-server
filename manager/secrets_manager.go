package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
)

type Provisioner interface {
	GetResources(ctx context.Context, resources []string) ([]auth.Secret, error)
}

type chanTuple struct {
	secrets chan<- []auth.Secret
	cancel  chan struct{}
}

type SecretsManager struct {
	Logger         *slog.Logger
	Provisioner    Provisioner
	RotationPeriod time.Duration

	sessionChans sync.Map
}

func (m *SecretsManager) Register(sessionID string, resourceNames []string) <-chan []auth.Secret {
	m.Logger.Info("registering session in secrets manager", slog.String("session_id", sessionID), slog.String("resources", fmt.Sprintf("%v", resourceNames)))

	secrets := make(chan []auth.Secret)
	chans := &chanTuple{
		secrets: secrets,
		cancel:  make(chan struct{}),
	}
	m.sessionChans.Store(sessionID, chans)

	go m.rotateSecrets(sessionID, resourceNames, chans)

	return secrets
}

func (m *SecretsManager) Unregister(sessionID string) error {
	val, loaded := m.sessionChans.LoadAndDelete(sessionID)
	if !loaded {
		return fmt.Errorf("session %s is not registered", sessionID)
	}

	coll := val.(*chanTuple)
	close(coll.cancel)

	return nil
}

func (m *SecretsManager) rotateSecrets(sessionID string, resourceNames []string, chans *chanTuple) {
	l := m.Logger.With(slog.String("session_id", sessionID))
	if err := m.sendSecrets(sessionID, resourceNames, chans); err != nil {
		l.Error("failed to get secrets", slog.Any("error", err))
		return
	}

	t := time.NewTicker(m.RotationPeriod)
	defer t.Stop()
	defer close(chans.secrets)

	for {
		select {
		case <-t.C:
			l.Info(fmt.Sprintf("scheduled secret update for resources: %v", resourceNames))
			if err := m.sendSecrets(sessionID, resourceNames, chans); err != nil {
				l.Error("failed to rotate secrets", slog.Any("error", err))
			}
		case <-chans.cancel:
			return
		}
	}
}

func (m *SecretsManager) sendSecrets(sessionID string, resourceNames []string, chans *chanTuple) error {
	ctx := context.WithValue(context.Background(), "session_id", sessionID)
	secrets, err := m.Provisioner.GetResources(ctx, resourceNames)
	if err != nil {
		return err
	}

	chans.secrets <- secrets
	return nil
}
