package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	pb "github.com/dzoniops/common/pkg/accommodation"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/timeout"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	stdout "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// interceptorLogger adapts go-kit logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func interceptorLogger(l log.Logger) logging.Logger {
	return logging.LoggerFunc(
		func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
			largs := append([]any{"msg", msg}, fields...)
			switch lvl {
			case logging.LevelDebug:
				_ = level.Debug(l).Log(largs...)
			case logging.LevelInfo:
				_ = level.Info(l).Log(largs...)
			case logging.LevelWarn:
				_ = level.Warn(l).Log(largs...)
			case logging.LevelError:
				_ = level.Error(l).Log(largs...)
			default:
				panic(fmt.Sprintf("unknown level %v", lvl))
			}
		},
	)
}

type AccommodationClient struct {
	client pb.AccommodationServiceClient
}

func InitClient(url string) *AccommodationClient {
	// Setup logging.
	logger := log.NewLogfmtLogger(os.Stderr)
	rpcLogger := log.With(logger, "service", "gRPC/client", "component", "accommodation-client")
	logTraceID := func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}

	// Setup metrics.
	reg := prometheus.NewRegistry()
	clMetrics := grpcprom.NewClientMetrics(
		grpcprom.WithClientHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets(
				[]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120},
			),
		),
	)
	reg.MustRegister(clMetrics)
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	// Set up OTLP tracing (stdout for debug).
	exporter, err := stdout.New(stdout.WithPrettyPrint())
	if err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)
	defer func() { _ = exporter.Shutdown(context.Background()) }()

	conn, err := grpc.Dial(
		url,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			timeout.UnaryClientInterceptor(500*time.Millisecond),
			otelgrpc.UnaryClientInterceptor(),
			clMetrics.UnaryClientInterceptor(grpcprom.WithExemplarFromContext(exemplarFromContext)),
			logging.UnaryClientInterceptor(
				interceptorLogger(rpcLogger),
				logging.WithFieldsFromContext(logTraceID),
			),
		),
		grpc.WithChainStreamInterceptor(
			otelgrpc.StreamClientInterceptor(),
			clMetrics.StreamClientInterceptor(
				grpcprom.WithExemplarFromContext(exemplarFromContext),
			),
			logging.StreamClientInterceptor(
				interceptorLogger(rpcLogger),
				logging.WithFieldsFromContext(logTraceID),
			),
		),
	)
	if err != nil {
		level.Error(logger).Log("err", err)
	}
	client := pb.NewAccommodationServiceClient(conn)
	g := &run.Group{}
	httpSrv := &http.Server{Addr: fmt.Sprintf(":%s", os.Getenv("PORT"))}
	g.Add(func() error {
		m := http.NewServeMux()
		// Create HTTP handler for Prometheus metrics.
		m.Handle("/metrics", promhttp.HandlerFor(
			reg,
			promhttp.HandlerOpts{
				// Opt into OpenMetrics e.g. to support exemplars.
				EnableOpenMetrics: true,
			},
		))
		httpSrv.Handler = m
		level.Info(logger).Log("msg", "starting HTTP server", "addr", httpSrv.Addr)
		return httpSrv.ListenAndServe()
	}, func(error) {
		if err := httpSrv.Close(); err != nil {
			level.Error(logger).Log("msg", "failed to stop web server", "err", err)
		}
	})

	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))

	if err := g.Run(); err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
	return &AccommodationClient{client: client}
}

func (c *AccommodationClient) GetAccommodation(
	ctx context.Context,
	id int64,
) (*pb.AccommodationInfo, error) {
	res, err := c.client.GetAccommodationById(ctx, &pb.AccommodationResponse{AccommodationId: id})
	if err != nil {
		return nil, err
	}
	return res, nil
}
