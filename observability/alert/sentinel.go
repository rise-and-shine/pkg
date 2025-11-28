package alert

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
	sentinelpb "github.com/code19m/sentinel/pb"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// SentinelProvider implements the Provider interface for sending alerts to Sentinel.
// It manages the gRPC client and connection to the Sentinel service.
type SentinelProvider struct {
	// cfg holds the configuration for the Sentinel provider.
	cfg Config
	// serviceName is the name of the service sending alerts.
	serviceName string
	// serviceVersion is the version of the service sending alerts.
	serviceVersion string
	// client is the gRPC client for the Sentinel service.
	client sentinelpb.SentinelServiceClient
	// conn is the gRPC client connection to the Sentinel service.
	conn *grpc.ClientConn
}

// NewSentinelProvider creates a new SentinelProvider instance.
// It establishes a gRPC connection to the Sentinel service specified in the cfg.
// serviceName and serviceVersion are used to identify the source of the alerts.
// If cfg.Disable is true, it returns a disabled provider that will not send alerts.
// Returns an error if the gRPC connection to Sentinel cannot be established.
func NewSentinelProvider(cfg Config, serviceName, serviceVersion string) (*SentinelProvider, error) {
	if cfg.Disable {
		return &SentinelProvider{cfg: cfg}, nil
	}

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:%d", cfg.SentinelHost, cfg.SentinelPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, errx.Wrap(err)
	}

	return &SentinelProvider{
		cfg:            cfg,
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		client:         sentinelpb.NewSentinelServiceClient(conn),
		conn:           conn,
	}, nil
}

// SendError sends an error report to the Sentinel service.
// It uses a context with a timeout defined by cfg.SendTimeout.
// If cfg.Disable is true, this method does nothing and returns nil.
// The serviceVersion from the SentinelProvider is automatically added to the details map.
//
// Example:
//
//	details := map[string]string{"user_id": "123", "request_id": "abc"}
//	err := provider.SendError(context.Background(), "DB_CONN_ERROR", "Failed to connect to database", "user_login", details)
//	if err != nil {
//		log.Printf("Failed to send alert: %v", err)
//	}
//
// Returns an error if the gRPC call to Sentinel fails or if the context times out.
func (sp *SentinelProvider) SendError(
	ctx context.Context,
	errCode, msg, operation string,
	details map[string]string,
) error {
	if sp.cfg.Disable {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sp.cfg.SendTimeout)
	defer cancel()

	if details == nil {
		details = make(map[string]string)
	}
	details["service_version"] = sp.serviceVersion

	_, err := sp.client.SendError(ctx, &sentinelpb.ErrorInfo{
		Code:      errCode,
		Message:   msg,
		Service:   sp.serviceName,
		Operation: operation,
		Details:   details,
	})

	return errx.Wrap(err)
}

// Close closes the gRPC connection to the Sentinel service.
// It should be called when the SentinelProvider is no longer needed to release resources.
// Returns an error if closing the connection fails.
func (sp *SentinelProvider) Close() error {
	if sp.conn != nil {
		return sp.conn.Close()
	}
	return nil
}
