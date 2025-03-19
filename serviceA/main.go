package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	opensemconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/middleware/http"
	"github.com/openzipkin/zipkin-go/model"
	zipikinreporter "github.com/openzipkin/zipkin-go/reporter/http"
)

type cep struct {
	Cep string `json:"cep"`
}

func typeofObject(variable interface{}) string {
	return fmt.Sprintf("%T", variable)
}

var OtelTracer trace.Tracer
var zipkinClient *zipkinhttp.Client

func startOtel(ctx context.Context) {
	res, err := resource.New(ctx, resource.WithAttributes(
		opensemconv.ServiceNameKey.String("serviceA"),
	),
	)
	if err != nil {
		slog.Error("startOtel", "Contexterr", "failed to create context")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.NewClient("otel-collector:4317", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("startOtel", "GRPCerr", "failed to create GRPC conection")
	}
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		slog.Error("startOtel", "Traceerr", "failed to create trace exporter")
	}
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(
		sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	OtelTracer = otel.Tracer("tracer")

}

func main() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	startOtel(ctx)

	reporter := zipikinreporter.NewReporter("http://zipkin-all-in-one:9411/api/v2/spans")
	serviceAEndpoint := &model.Endpoint{
		ServiceName: "serviceA",
		IPv4:        getOutboundIP(),
		Port:        8080}
	sampler, err := zipkin.NewCountingSampler(1)
	if err != nil {
		log.Fatal(err)
	}
	tracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(serviceAEndpoint), zipkin.WithSampler(sampler))
	if err != nil {
		log.Fatal(err)
	}

	ctx = context.Background()
	ctx, span := OtelTracer.Start(ctx, "serviceA")
	defer span.End()

	zipkinClient, err = zipkinhttp.NewClient(tracer, zipkinhttp.ClientTrace(true))
	if err != nil {
		log.Fatal(err)
	}
	router := http.NewServeMux()
	serverMidleware := zipkinhttp.NewServerMiddleware(tracer, zipkinhttp.TagResponseSize(true))

	http.Handle("/", serverMidleware(router))
	router.HandleFunc("POST /", cepValidatorHandler)

	http.ListenAndServe(":8080", router)

	slog.Info("serviceA started")
	select {
	case <-channel:
		slog.Info("serviceA was stopped")
	case <-ctx.Done():
		slog.Info("serviceA stopped naturally")
	}
}
func cepValidatorHandler(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := OtelTracer.Start(ctx, "serviceA")
	defer span.End()


	zspan := zipkin.SpanFromContext(r.Context())
	ctx = zipkin.NewContext(ctx, zspan)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var cep cep
	if error := json.Unmarshal(body, &cep); error != nil {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	cepValue := cep.Cep

	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte(cepValue))

	typecep := typeofObject(cep.Cep)
	// validades cep
	if len(cepValue) == 8 && typecep == "string" {

		url := "http://localhost:8081/?cep=" + cepValue
		req, err := http.NewRequestWithContext(ctx, "get", url, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

		res, err := zipkinClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			slog.Error(err.Error())
		}
		slog.Info("status", "code", res.StatusCode)
		switch res.StatusCode {
		case http.StatusOK:
			body, err := io.ReadAll(res.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer res.Body.Close()
			sbody := string(body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sbody))
	case http.StatusNotFound:
		http.Error(w, "not found", http.StatusNotFound)
	case http.StatusUnprocessableEntity:
		http.Error(w, "internal server error", http.StatusUnprocessableEntity)
	}
	}
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
