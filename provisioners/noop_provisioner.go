package provisioners

import (
	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
)

// NoopProvisioner is useful for testing purposes
type NoopProvisioner struct{}

func (*NoopProvisioner) GetResources(resources []string) []auth.Secret {
	secrets := make([]auth.Secret, 0, len(resources))
	for _, n := range resources {
		secrets = append(secrets, auth.Secret{
			Name: n,
		})
	}

	return secrets
}
