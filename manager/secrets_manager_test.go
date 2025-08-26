package manager

import (
	"log/slog"
	"os"
	"testing"
	"time"

	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/stretchr/testify/mock"
)

func TestSecretsManager(t *testing.T) {
	p := &MockProvisioner{}
	sm := &SecretsManager{
		Logger:         slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		Provisioner:    p,
		RotationPeriod: 20 * time.Second, // value big enough so that there's no additional rotation during the test
	}

	// Provisioner should be called exactly once during the test
	p.On("GetResources", mock.Anything, mock.Anything).Return([]auth.Secret{}, nil).Once()

	// Register a session
	sessionID := "test-session"
	secrets := sm.Register(sessionID, []string{"cert1", "cert2"})

	v, _ := sm.sessionChans.Load(sessionID)
	if v == nil {
		t.Fatalf("expected session %q to be registered", sessionID)
	}

	// Check that secrets are rotated
	select {
	case <-secrets:
	case <-time.After(sm.RotationPeriod):
		t.Fatal("expected secrets to be sent")
	}

	// Unregister the session
	if err := sm.Unregister(sessionID); err != nil {
		t.Fatalf("failed to unregister session: %v", err)
	}

	v, _ = sm.sessionChans.Load(sessionID)
	if v != nil {
		t.Fatalf("expected session %q to be unregistered", sessionID)
	}

	// Check that secrets channel is closed
	select {
	case _, ok := <-secrets:
		if ok {
			t.Fatal("expected secrets channel to be closed after unregistering")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for secrets channel to close")
	}

	if !p.AssertExpectations(t) {
		t.Fatal("provisioner calls did not match expectations")
	}
}
