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
	neturl "net/url"
	"os"
	"os/signal"
	"strings"
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

type ViaCEP struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}
type Weather struct {
	Location struct {
		Name           string  `json:"name"`
		Region         string  `json:"region"`
		Country        string  `json:"country"`
		Lat            float64 `json:"lat"`
		Lon            float64 `json:"lon"`
		TzID           string  `json:"tz_id"`
		LocaltimeEpoch int     `json:"localtime_epoch"`
		Localtime      string  `json:"localtime"`
	} `json:"location"`
	Current struct {
		LastUpdatedEpoch int     `json:"last_updated_epoch"`
		LastUpdated      string  `json:"last_updated"`
		TempC            float64 `json:"temp_c"`
		TempF            float64 `json:"temp_f"`
		IsDay            int     `json:"is_day"`
		Condition        struct {
			Text string `json:"text"`
			Icon string `json:"icon"`
			Code int    `json:"code"`
		} `json:"condition"`
		WindMph    float64 `json:"wind_mph"`
		WindKph    float64 `json:"wind_kph"`
		WindDegree int     `json:"wind_degree"`
		WindDir    string  `json:"wind_dir"`
		PressureMb float64 `json:"pressure_mb"`
		PressureIn float64 `json:"pressure_in"`
		PrecipMm   float64 `json:"precip_mm"`
		PrecipIn   float64 `json:"precip_in"`
		Humidity   int     `json:"humidity"`
		Cloud      int     `json:"cloud"`
		FeelslikeC float64 `json:"feelslike_c"`
		FeelslikeF float64 `json:"feelslike_f"`
		WindchillC float64 `json:"windchill_c"`
		WindchillF float64 `json:"windchill_f"`
		HeatindexC float64 `json:"heatindex_c"`
		HeatindexF float64 `json:"heatindex_f"`
		DewpointC  float64 `json:"dewpoint_c"`
		DewpointF  float64 `json:"dewpoint_f"`
		VisKm      float64 `json:"vis_km"`
		VisMiles   float64 `json:"vis_miles"`
		Uv         float64 `json:"uv"`
		GustMph    float64 `json:"gust_mph"`
		GustKph    float64 `json:"gust_kph"`
	} `json:"current"`
}

var OtelTracer trace.Tracer
var zipkinClient *zipkinhttp.Client

func startOtel(ctx context.Context) {
	res, err := resource.New(ctx, resource.WithAttributes(
		opensemconv.ServiceNameKey.String("serviceB"),
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
	serviceBEndpoint := &model.Endpoint{
		ServiceName: "serviceB",
		IPv4:        getOutboundIP(),
		Port:        8081}
	sampler, err := zipkin.NewCountingSampler(1)
	if err != nil {
		log.Fatal(err)
	}
	tracer, err := zipkin.NewTracer(reporter, zipkin.WithLocalEndpoint(serviceBEndpoint), zipkin.WithSampler(sampler))
	if err != nil {
		log.Fatal(err)
	}

	ctx = context.Background()
	ctx, span := OtelTracer.Start(ctx, "service B")
	defer span.End()

	zipkinClient, err = zipkinhttp.NewClient(tracer, zipkinhttp.ClientTrace(true))
	if err != nil {
		log.Fatal(err)
	}
	router := http.NewServeMux()
	serverMidleware := zipkinhttp.NewServerMiddleware(tracer, zipkinhttp.TagResponseSize(true))

	http.Handle("/", serverMidleware(router))
	router.HandleFunc("/", testcep)

	http.ListenAndServe(":8081", nil)

	slog.Info("serviceB ended")
	select {
	case <-channel:
		slog.Info("serviceB was stopped")
	case <-ctx.Done():
		slog.Info("serviceB stopped naturally")
	}
}

func testcep(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := OtelTracer.Start(ctx, "serviceB")
	defer span.End()

	zspan := zipkin.SpanFromContext(r.Context())
	ctx = zipkin.NewContext(ctx, zspan)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cep := r.URL.Query().Get("cep")
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(r.Header))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"cep": cep,
	})
}

func cepHandler(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := OtelTracer.Start(ctx, "serviceB")
	defer span.End()

	zspan := zipkin.SpanFromContext(r.Context())
	ctx = zipkin.NewContext(ctx, zspan)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cep := r.URL.Query().Get("cep")

	// request para pegar localidade
	// temp
	// status
	// message
	// err
	url := "http://viacep.com.br/ws/" + cep + "/json/"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
//injection
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	switch res.StatusCode {
	case http.StatusBadRequest:
		err = fmt.Errorf("invalid  zipcode in viacep")
		fmt.Println(err)
		os.Exit(1)
	case http.StatusNotFound:
		err = fmt.Errorf("can not find zipcode in viacep")
		fmt.Println(err)
		os.Exit(1)
	case http.StatusOK:
		body, err := io.ReadAll(res.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		defer res.Body.Close()
		if strings.Contains(string(body), `"erro":"true"`) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}

		var data ViaCEP
		err = json.Unmarshal(body, &data)
		if err != nil {
			log.Fatal(err)
		}
		local := data.Localidade
		url2 := "http://api.weatherapi.com/v1/current.json?key=18525c8de5ac479f994185201250303&q=" + neturl.QueryEscape(local) + "&aqi=no"

		req2, err := http.NewRequestWithContext(ctx, "GET", url2, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		res2, err := zipkinClient.Do(req2)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		switch res2.StatusCode {
		case http.StatusBadRequest:
			err = fmt.Errorf("invalid  zipcode in WeatherAPI")
			fmt.Println(err)
			os.Exit(1)
		case http.StatusNotFound:
			err = fmt.Errorf("can not find zipcode in WeatherAPI")
			fmt.Println(err)
			os.Exit(1)
		case http.StatusOK:
			body2, err := io.ReadAll(res2.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			defer res2.Body.Close()
			if strings.Contains(string(body2), `"error":"true"`) {
				http.Error(w, "Not Found", http.StatusNotFound)
			}

			var data2 Weather
			err = json.Unmarshal(body2, &data2)
			if err != nil {
				log.Fatal(err)
			}
			tempC := data2.Current.TempC
			tempF := tempC*1.8 + 32
			tempK := tempC + 273

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"city":   local,
				"temp_C": tempC,
				"temp_F": tempF,
				"temp_K": tempK,
			})

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
