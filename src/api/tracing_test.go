package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

func preserveTracing(t *testing.T) {
	t.Helper()
	provider, propagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
	})
}

func clearTracingEnvironment(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "OTEL_") {
			t.Setenv(name, "")
		}
	}
}

func TestTracingDisabledWithoutCollector(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(fmt.Sprint(disabled), func(t *testing.T) {
			clearTracingEnvironment(t)
			if disabled {
				t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
				t.Setenv("OTEL_SDK_DISABLED", "true")
			}
			previous := otel.GetTracerProvider()
			shutdown, err := initTracing(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if otel.GetTracerProvider() != previous {
				t.Fatal("disabled tracing installed a provider")
			}
			if err := shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInvalidTracingProtocol(t *testing.T) {
	clearTracingEnvironment(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/json")
	if _, err := initTracing(context.Background()); err == nil || !strings.Contains(err.Error(), "unsupported OTLP traces protocol") {
		t.Fatalf("invalid protocol must fail explicitly: %v", err)
	}
}

func TestRequestAndCosmosTracePropagation(t *testing.T) {
	preserveTracing(t)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	const traceID = "0123456789abcdef0123456789abcdef"
	const parentID = "0123456789abcdef"
	downstreamParents := make(chan string, 2)
	cosmos := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			downstreamParents <- r.Header.Get("traceparent")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ms-item-count", "0")
		_, _ = io.WriteString(w, `{"Documents":[],"_count":0}`)
	}))
	defer cosmos.Close()
	key := base64.StdEncoding.EncodeToString([]byte("local-test-key"))
	store, err := newCosmosStore(cosmosConfig{Endpoint: cosmos.URL, Key: key, Database: "catbooks", Container: "books", AllowHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	h := handler(store, "test-password")
	if w := request(h, "GET", "/healthz", "", ""); w.Code != http.StatusOK {
		t.Fatalf("health check: %d %s", w.Code, w.Body.String())
	}
	if len(recorder.Ended()) != 0 {
		t.Fatal("health probes generated trace noise")
	}
	if parent := <-downstreamParents; parent != "" {
		t.Fatal("health probe propagated a trace to Cosmos")
	}
	r := httptest.NewRequest("GET", "/api/books", nil)
	r.Header.Set("traceparent", "00-"+traceID+"-"+parentID+"-01")
	r.SetBasicAuth("admin", "test-password")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("list books: %d %s", w.Code, w.Body.String())
	}
	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected API and Cosmos HTTP spans, got %d", len(spans))
	}
	client, server := spans[0], spans[1]
	if server.Name() != "GET /api/books" || server.SpanKind() != trace.SpanKindServer ||
		server.SpanContext().TraceID().String() != traceID || server.Parent().SpanID().String() != parentID {
		t.Fatalf("incoming W3C trace context or route name was lost: %+v", server)
	}
	if client.SpanKind() != trace.SpanKindClient || client.Parent().SpanID() != server.SpanContext().SpanID() ||
		client.SpanContext().TraceID() != server.SpanContext().TraceID() {
		t.Fatal("Cosmos request is not a child of the API span")
	}
	if downstreamParent := <-downstreamParents; downstreamParent != "00-"+traceID+"-"+client.SpanContext().SpanID().String()+"-01" {
		t.Fatalf("Cosmos request did not receive the client trace context: %q", downstreamParent)
	}
	for _, span := range spans {
		attributes := fmt.Sprint(span.Attributes())
		if strings.Contains(attributes, "test-password") || strings.Contains(attributes, key) || strings.Contains(strings.ToLower(attributes), "authorization") {
			t.Fatal("credentials were captured in span attributes")
		}
	}
}

func TestTracingErrorAndParameterizedRoute(t *testing.T) {
	preserveTracing(t)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer provider.Shutdown(context.Background())
	otel.SetTracerProvider(provider)
	h := handler(&memoryStore{fail: true}, "test-password")
	if w := request(h, "DELETE", "/api/books/arbitrary-book-id", "", "test-password"); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("database failure status changed: %d", w.Code)
	}
	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "DELETE /api/books/{id}" || spans[0].Status().Code != codes.Error {
		t.Fatalf("missing error status or low-cardinality route name: %+v", spans)
	}
}

type traceExport struct {
	request *collectortrace.ExportTraceServiceRequest
	header  string
}

type grpcTraceCollector struct {
	collectortrace.UnimplementedTraceServiceServer
	exports chan traceExport
}

func (c *grpcTraceCollector) Export(ctx context.Context, request *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	headers, _ := metadata.FromIncomingContext(ctx)
	c.exports <- traceExport{request, strings.Join(headers.Get("x-demo-key"), ",")}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func TestOTLPExportAndShutdown(t *testing.T) {
	for _, protocol := range []string{"grpc", "http/protobuf"} {
		t.Run(protocol, func(t *testing.T) {
			preserveTracing(t)
			clearTracingEnvironment(t)
			exports := make(chan traceExport, 1)
			var endpoint string
			if protocol == "grpc" {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := grpc.NewServer()
				collectortrace.RegisterTraceServiceServer(server, &grpcTraceCollector{exports: exports})
				go func() {
					if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
						t.Error(err)
					}
				}()
				t.Cleanup(server.Stop)
				endpoint = "http://" + listener.Addr().String()
				t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol)
			} else {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/v1/traces" || r.Header.Get("Content-Type") != "application/x-protobuf" {
						t.Errorf("unexpected OTLP request: %s %s", r.Method, r.URL.Path)
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					request := new(collectortrace.ExportTraceServiceRequest)
					if err := proto.Unmarshal(body, request); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					exports <- traceExport{request, r.Header.Get("x-demo-key")}
					w.Header().Set("Content-Type", "application/x-protobuf")
				}))
				t.Cleanup(server.Close)
				endpoint = server.URL + "/v1/traces"
				t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
				t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", protocol)
			}
			if protocol == "grpc" {
				t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
				t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "x-demo-key=local-test-value")
			} else {
				t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
				t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", endpoint)
				t.Setenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS", "x-demo-key=local-test-value")
			}
			t.Setenv("OTEL_SERVICE_NAME", "catbooks-test")
			shutdown, err := initTracing(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			_, span := otel.Tracer("catbooks/api").Start(context.Background(), "export-check")
			span.End()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case exported := <-exports:
				resources := exported.request.ResourceSpans
				if len(resources) != 1 || len(resources[0].ScopeSpans) != 1 || len(resources[0].ScopeSpans[0].Spans) != 1 ||
					resources[0].ScopeSpans[0].Spans[0].Name != "export-check" {
					t.Fatalf("unexpected exported traces: %v", exported.request)
				}
				var serviceName string
				for _, attr := range resources[0].Resource.Attributes {
					if attr.Key == "service.name" {
						serviceName = attr.Value.GetStringValue()
					}
				}
				if serviceName != "catbooks-test" || exported.header != "local-test-value" {
					t.Fatal("OTLP service identity or collector authentication header was lost")
				}
			case <-ctx.Done():
				t.Fatal("shutdown did not flush the pending trace")
			}
		})
	}
}
