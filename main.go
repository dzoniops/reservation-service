package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"syscall"

	pb "github.com/dzoniops/common/pkg/reservation"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/joho/godotenv"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dzoniops/reservation-service/db"
	"github.com/dzoniops/reservation-service/services"
	"github.com/dzoniops/reservation-service/utils"
)

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

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	db.InitDB()
	utils.InitValidator()

	// Setup logging.
	logger := log.NewLogfmtLogger(os.Stderr)
	rpcLogger := log.With(logger, "service", "gRPC/client", "component", "reservation")
	logTraceID := func(ctx context.Context) logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return logging.Fields{"traceID", span.TraceID().String()}
		}
		return nil
	}

	// Setup metrics.
	srvMetrics := grpcprom.NewServerMetrics(
		grpcprom.WithServerHandlingTimeHistogram(
			grpcprom.WithHistogramBuckets(
				[]float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120},
			),
		),
	)
	reg := prometheus.NewRegistry()
	reg.MustRegister(srvMetrics)
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{"traceID": span.TraceID().String()}
		}
		return nil
	}

	// Setup metric for panic recoveries.
	panicsTotal := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})
	grpcPanicRecoveryHandler := func(p any) (err error) {
		panicsTotal.Inc()
		level.Error(rpcLogger).
			Log("msg", "recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	grpcSrv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			// Order matters e.g. tracing interceptor have to create span first for the later exemplars to work.
			otelgrpc.UnaryServerInterceptor(),
			srvMetrics.UnaryServerInterceptor(
				grpcprom.WithExemplarFromContext(exemplarFromContext),
			),
			logging.UnaryServerInterceptor(
				interceptorLogger(rpcLogger),
				logging.WithFieldsFromContext(logTraceID),
			),
			// selector.UnaryServerInterceptor(auth.UnaryServerInterceptor(authFn), selector.MatchFunc(allButHealthZ)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)),
		),
		grpc.ChainStreamInterceptor(
			otelgrpc.StreamServerInterceptor(),
			srvMetrics.StreamServerInterceptor(
				grpcprom.WithExemplarFromContext(exemplarFromContext),
			),
			logging.StreamServerInterceptor(
				interceptorLogger(rpcLogger),
				logging.WithFieldsFromContext(logTraceID),
			),
			// selector.StreamServerInterceptor(auth.StreamServerInterceptor(authFn), selector.MatchFunc(allButHealthZ)),
			recovery.StreamServerInterceptor(
				recovery.WithRecoveryHandler(grpcPanicRecoveryHandler),
			),
		),
	)

	pb.RegisterReservationServiceServer(
		grpcSrv,
		&services.Server{},
	)
	srvMetrics.InitializeMetrics(grpcSrv)

	g := &run.Group{}
	g.Add(func() error {
		l, err := net.Listen("tcp", fmt.Sprintf(":%s", os.Getenv("PORT")))
		if err != nil {
			return err
		}
		level.Info(logger).Log("msg", "starting gRPC server", "addr", l.Addr().String())
		return grpcSrv.Serve(l)
	}, func(err error) {
		grpcSrv.GracefulStop()
		grpcSrv.Stop()
	})

	httpSrv := &http.Server{Addr: fmt.Sprintf(":%s", os.Getenv("METRICS_PORT"))}
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
}
