package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

type Provider struct {
	traces  *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider
	tracer  trace.Tracer
	meter   metric.Meter
}

func New(ctx context.Context, serviceName, serviceVersion string) (*Provider, error) {
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.ServiceNamespace("identity"),
	))
	if err != nil {
		return nil, err
	}
	traces := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)
	metrics := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))),
	)
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &Provider{
		traces: traces, metrics: metrics,
		tracer: traces.Tracer(serviceName), meter: metrics.Meter(serviceName),
	}, nil
}

func (p *Provider) Shutdown(ctx context.Context) error {
	traceErr := p.traces.Shutdown(ctx)
	metricErr := p.metrics.Shutdown(ctx)
	if traceErr != nil {
		return traceErr
	}
	return metricErr
}

func (p *Provider) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	counters := map[string]metric.Int64Counter{}
	histograms := map[string]metric.Float64Histogram{}
	return func(c *echo.Context) error {
		endpoint := endpointName(c.Request().URL.Path)
		if endpoint == "" {
			return next(c)
		}
		counter := counters[endpoint]
		if counter == nil {
			counter, _ = p.meter.Int64Counter("oauth2_" + endpoint + "_requests_total")
			counters[endpoint] = counter
		}
		histogram := histograms[endpoint]
		if histogram == nil {
			histogram, _ = p.meter.Float64Histogram(
				"oauth2_"+endpoint+"_request_duration_seconds",
				metric.WithUnit("s"),
			)
			histograms[endpoint] = histogram
		}

		req := c.Request()
		parent := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		ctx, span := p.tracer.Start(parent, "http."+req.Method+" /"+endpoint,
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				semconv.URLPath(req.URL.Path),
			))
		c.SetRequest(req.WithContext(ctx))
		start := time.Now()
		err := next(c)
		status := http.StatusOK
		if response, ok := c.Response().(*echo.Response); ok && response.Status != 0 {
			status = response.Status
		}
		result := "success"
		if err != nil || status >= http.StatusBadRequest {
			result = "error"
		}
		attrs := metric.WithAttributes(
			attribute.String("result", result),
			attribute.Int("http.response.status_code", status),
		)
		counter.Add(ctx, 1, attrs)
		histogram.Record(ctx, time.Since(start).Seconds(), attrs)
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if err != nil {
			span.RecordError(err)
		}
		span.End()
		return err
	}
}

func endpointName(path string) string {
	switch {
	case strings.HasPrefix(path, "/authorize"):
		return "authorize"
	case strings.HasPrefix(path, "/par"):
		return "par"
	case strings.HasPrefix(path, "/token"):
		return "token"
	case strings.HasPrefix(path, "/introspect"):
		return "introspect"
	case strings.HasPrefix(path, "/revoke"):
		return "revoke"
	case strings.HasPrefix(path, "/userinfo"):
		return "userinfo"
	case strings.HasPrefix(path, "/jwks"):
		return "jwks"
	case strings.HasPrefix(path, "/register"):
		return "register"
	case strings.HasPrefix(path, "/device_authorization"):
		return "device_authorization"
	case strings.HasPrefix(path, "/.well-known/"):
		return "discovery"
	default:
		return ""
	}
}
