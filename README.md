# ACM Secret Discovery Server

ACM Secret Discovery Server is an Envoy [SDS](https://www.envoyproxy.io/docs/envoy/latest/configuration/security/secret) interface server that provides TLS certificates to Envoy proxy using AWS Certificate Manager (ACM).

It provides certificates for plain and mutual TLS (mTLS).

## Intro 

The Service designed to run as a sidecar in an Envoy proxy Kubernetes Pod. Allowing to dynamically configure TLS and optionally mTLS for downstream connections. Communication with Envoy is via gRPC over Unix Domain Socket(UDS) and implements only State of the World(SotW) xDS protocol.

Public TLS certificates are obtained from AWS Certificate Manager (ACM) using the [exportable certificates](https://docs.aws.amazon.com/acm/latest/userguide/acm-exportable-certificates.html) feature.

An optional CA can be provided for the mTLS validation context. It is loaded as-is from an environment variable.

## Usage

Configuration is through environment variables. AWS credentials are obtained using standard SDK means, e.g. [pod identity](https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html), [instance profile](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html), environment variables, [or others](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-gosdk.html).

- `CERTIFICATE_ARN` - required
- `CA_CERT` - optional - CA certificate in PEM format for mTLS - not the file path.
- `ROTATION_PERIOD` - optional - duration string - how often to refresh the certificates - default `24h`

## Motivation

We've created this service to simplify TLS and mTLS configuration for Envoy proxies running in Kubernetes. 

Especially mTLS is a security requirement, that can't be offloaded to AWS NLBs at the time of writing. So TLS termination had to be moved into Envoy. AWS ACM is a great service for certificate management and we can offload the complexity of handling certificate issuance and renewal - even more so with the recently launch of exportable public certificates.

This service is now a corner stone for our gRPC based workloads. It provides secure communication between our CDN and the backend services.

## Internals

Names for secrets are hardcoded at the moment. Certificate chain and private key can be obtained by requesting `certificate` secret. And CA certificate can be obtained by requesting `ca_certificate` secret.

The certificate is refreshed periodically, based on the `ROTATION_PERIOD` environment variable. ACM exportable certificates are valid for 13 months and for simplicity, there is no cache, watch or hash comparison in this service. Every tick will export the certificate again from ACM and return it to Envoy. 

## Costs

AWS ACM exportable public certificates incur charges on issuance, renewal and export. Please see the [pricing page](https://aws.amazon.com/certificate-manager/pricing/) for details.

## See also

- [AWS Certificate Manager introduces exportable public SSL/TLS certificates to use anywhere](https://aws.amazon.com/blogs/aws/aws-certificate-manager-introduces-exportable-public-ssl-tls-certificates-to-use-anywhere/)

## Copyright

Provided without warranty of any kind. Licensed under Apache License 2.0 - see [LICENSE](LICENSE) for details.

Created by and &copy; Sixt SE - https://www.sixt.com

<a href="https://www.sixt.com">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset=".github/sixt_dark.png">
      <source media="(prefers-color-scheme: light)" srcset=".github/sixt_light.png">
      <img width="200px" alt="Sixt logo" src=".github/sixt_dark.png">
    </picture>
</a>
