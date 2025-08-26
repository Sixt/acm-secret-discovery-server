package provisioners

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/google/uuid"
)

const (
	certificateResource   = "certificate"
	caCertificateResource = "ca_certificate"
)

var acmResources = map[string]bool{
	certificateResource:   true,
	caCertificateResource: true,
}

// ACMProvisioner is useful for testing purposes
type ACMProvisioner struct {
	Logger *slog.Logger
	Client *acm.Client

	CertificateARN string
	CACert         string
}

func (p *ACMProvisioner) GetResources(ctx context.Context, resources []string) (secrets []auth.Secret, err error) {
	if err = validateResources(resources); err != nil {
		return nil, fmt.Errorf("invalid resources: %w", err)
	}

	for _, r := range resources {
		switch r {
		case certificateResource:
			s, err := p.getCertificateSecret(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get certificate secret: %w", err)
			}

			p.Logger.Info("Certificate secret retrieved successfully")
			secrets = append(secrets, s)
		case caCertificateResource:
			s, err := p.getCACertificateSecret(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get CA certificate secret: %w", err)
			}

			p.Logger.Info("CA certificate secret retrieved successfully")
			secrets = append(secrets, s)
		}
	}

	return secrets, nil
}

func (p *ACMProvisioner) getCertificateSecret(ctx context.Context) (auth.Secret, error) {
	passphrase := base64.StdEncoding.EncodeToString([]byte(uuid.NewString()))
	out, err := p.Client.ExportCertificate(ctx, &acm.ExportCertificateInput{
		CertificateArn: &p.CertificateARN,
		Passphrase:     []byte(passphrase),
	})
	if err != nil {
		return auth.Secret{}, fmt.Errorf("failed to export certificate: %w", err)
	}

	return auth.Secret{
		Name: certificateResource,
		Type: &auth.Secret_TlsCertificate{
			TlsCertificate: &auth.TlsCertificate{
				CertificateChain: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: *out.Certificate + *out.CertificateChain,
					},
				},
				PrivateKey: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: *out.PrivateKey,
					},
				},
				Password: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: passphrase,
					},
				},
			},
		},
	}, nil
}

func (p *ACMProvisioner) getCACertificateSecret(ctx context.Context) (auth.Secret, error) {
	if p.CACert == "" {
		return auth.Secret{}, fmt.Errorf("CA certificate is not set")
	}

	return auth.Secret{
		Name: caCertificateResource,
		Type: &auth.Secret_ValidationContext{
			ValidationContext: &auth.CertificateValidationContext{
				TrustedCa: &core.DataSource{
					Specifier: &core.DataSource_InlineString{
						InlineString: p.CACert,
					},
				},
			},
		},
	}, nil
}

func validateResources(resources []string) error {
	for _, r := range resources {
		if !acmResources[r] {
			return fmt.Errorf("unknown resource: %q", r)
		}
	}

	return nil
}
