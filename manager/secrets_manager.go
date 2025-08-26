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
	m.Logger.Info("registering session in secrets manager", slog.String("session_id", sessionID))

	secrets := make(chan []auth.Secret)
	chans := &chanTuple{
		secrets: secrets,
		cancel:  make(chan struct{}),
	}
	m.sessionChans.Store(sessionID, chans)

	go m.rotateSecrets(resourceNames, chans)

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

func (m *SecretsManager) rotateSecrets(resourceNames []string, chans *chanTuple) {
	if err := m.sendSecrets(resourceNames, chans); err != nil {
		m.Logger.Error("failed to get initial secrets", slog.Any("error", err))
		return
	}

	t := time.NewTicker(m.RotationPeriod)
	defer t.Stop()
	defer close(chans.secrets)

	for {
		select {
		case <-t.C:
			if err := m.sendSecrets(resourceNames, chans); err != nil {
				m.Logger.Error("failed to rotate secrets", slog.Any("error", err))
			}
		case <-chans.cancel:
			return
		}
	}
}

func (m *SecretsManager) sendSecrets(resourceNames []string, chans *chanTuple) error {
	secrets, err := m.Provisioner.GetResources(context.Background(), resourceNames)
	if err != nil {
		return err
	}

	chans.secrets <- secrets
	return nil
}
