# Envoy Certificate Manager
This service implements [SDS](https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret) which allows dynamic certificate management.

Service intended to run as a sidecar in Envoy proxy container. Allowing to dynamically configure TLS/mTLS for downstream connections. Communication with Envoy is via gRPC over Unix Domain Socket(UDS) and implements only State of the World(SotW) xDS protocol.

[Exportable certificates API](https://docs.aws.amazon.com/acm/latest/userguide/export-private.html) from AWS Certificate Manager is used to obtain all requires certificates/keys.

Certificate ARN is required, it can be provided as a flag `certificate_arn` or as an environment variable `CERTIFICATE_ARN`.

Additionally CA certificate for validation context can be supply via flag `ca_cert` or as an environment variable `CA_CERT`.

Names for secrets are hardcoded, at the moment. Certificate chain and private key can be obtained by requesting `certificate` secret. And CA certificate can be obtained by requesting `ca_certificate` secret.
