package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	secretservice "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/kelseyhightower/envconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/sixt/acm-secret-discovery-server/config"
	"github.com/sixt/acm-secret-discovery-server/manager"
	"github.com/sixt/acm-secret-discovery-server/provisioners"
	"github.com/sixt/acm-secret-discovery-server/sds"
)

// Version value is set by the linker during the build
var Version string
var Commit string

func main() {
	cfg := &config.Config{}
	if err := envconfig.Process("", cfg); err != nil {
		slog.Info("ACM SDS server - version: " + Version + " commit: " + Commit)
		slog.Error("failed to process configuration", slog.Any("error", err))
		os.Exit(1)
	}

	logger := slog.
		New(slog.NewJSONHandler(os.Stdout, nil)).
		With(slog.Any("service", "acm-secret-discovery"))

	logger.Info("ACM SDS server starting - version: " + Version + " commit: " + Commit)

	// Create a context that will be stopped when the program receives an interrupt signal
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create secrets manager
	awscfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("failed to load AWS config", slog.Any("error", err))
		os.Exit(1)
	}
	acmClient := acm.NewFromConfig(awscfg)
	logger.Info("AWS ACM client initialized")

	logger.Info("config", slog.String("arn", cfg.CertificateARN), slog.String("ca_cert", cfg.CACert))
	secretsManager := &manager.SecretsManager{
		Logger: logger,
		Provisioner: &provisioners.ACMProvisioner{
			Logger: logger,
			Client: acmClient,

			CertificateARN: cfg.CertificateARN,
			CACert:         cfg.CACert,
		},
		RotationPeriod: cfg.RotationPeriod,
	}

	// attempt to fetch secrets to verify that everything is working
	if cfg.CACert == "" {
		_, err = secretsManager.Provisioner.GetResources(ctx, []string{"certificate"})
	} else {
		_, err = secretsManager.Provisioner.GetResources(ctx, []string{"certificate", "ca_certificate"})
	}
	if err != nil {
		logger.Error("failed to get secrets", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize SDS server
	sdsServerStop := make(chan struct{})
	sdsServer := &sds.Server{
		Logger:  logger,
		Stop:    sdsServerStop,
		Manager: secretsManager,
	}

	grpcServer := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
	reflection.Register(grpcServer)

	// Register SDS service
	secretservice.RegisterSecretDiscoveryServiceServer(grpcServer, sdsServer)

	// Start SDS server on UDS
	listener, err := net.Listen("unix", "/tmp/envoy.sock")
	if err != nil {
		logger.Error("failed to listen on UDS: /tmp/envoy.sock")
		os.Exit(1)
	}
	if err := os.Chmod("/tmp/envoy.sock", 0777); err != nil {
		logger.Error("failed to set permissions on UDS: /tmp/envoy.sock", slog.Any("error", err))
		os.Exit(1)
	}

	go func() {
		defer stop()
		if err := grpcServer.Serve(listener); err != nil {
			logger.Error("failed starting grpc server", slog.Any("error", err))
		}
		logger.Info("grpc server stopped")
	}()

	logger.Info("ready to serve")

	<-ctx.Done()
	logger.Info("graceful shutdown triggered")

	// Trying to make SDS server to stop running streams
	close(sdsServerStop)

	timer := time.AfterFunc(10*time.Second, func() {
		log.Println("server couldn't stop gracefully in time, doing force stop")
		grpcServer.Stop()
	})
	defer timer.Stop()

	grpcServer.GracefulStop()

	logger.Info("SDS server shut down gracefully")
}
