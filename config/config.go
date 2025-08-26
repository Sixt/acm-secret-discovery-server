package config

import (
	"time"
)

type Config struct {
	RotationPeriod time.Duration `envconfig:"ROTATION_PERIOD" default:"24h"`
	CertificateARN string        `envconfig:"CERTIFICATE_ARN" required:"true"`
	CACert         string        `envconfig:"CA_CERT"`
}
