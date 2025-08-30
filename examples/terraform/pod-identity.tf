data "aws_iam_policy_document" "envoy-edge-pod-identity" {
  statement {
    effect = "Allow"
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
    actions = [
      "sts:TagSession",
      "sts:AssumeRole"
    ]
  }
}

resource "aws_iam_role" "envoy-edge-pod-identity" {
  name               = "envoy-edge-pod-identity"
  assume_role_policy = data.aws_iam_policy_document.envoy-edge-pod-identity.json
}

resource "aws_eks_pod_identity_association" "example" {
  cluster_name    = aws_eks_cluster.example.name
  namespace       = "ingress"
  service_account = "envoy-edge"
  role_arn        = aws_iam_role.envoy-edge-pod-identity.arn
}

data "aws_acm_certificate" "grpc" {
  domain      = "envoy.sixt.com"
  statuses    = ["ISSUED"]
  most_recent = true
}

data "aws_iam_policy_document" "acm-secret-discovery-server" {
  statement {
    effect    = "Allow"
    actions   = ["acm:ExportCertificate"]
    resources = [data.aws_acm_certificate.grpc.arn]

    condition {
      test     = "StringLike"
      variable = "aws:PrincipalTag/kubernetes-pod-name"
      values   = ["envoy*"]
    }
  }
}

resource "aws_iam_role_policy" "acm-secret-discovery-server" {
  role   = aws_iam_role.envoy-edge-pod-identity.name
  name   = "acm-secret-discovery-server"
  policy = data.aws_iam_policy_document.acm-secret-discovery-server.json
}

