package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

)

const name = "serviceA"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	logger = otelslog.NewLogger(name)
	cep string
	typecep	string
)

func init(){
	var err error
	cep, err
}
func typeofObject(variable interface{}) string {
	return fmt.Sprintf("%T", variable)
 }
 
func cepValidatorHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context())
	defer span.End()

	cep := r.URL.Query().Get("cep")
	typecep:= typeofObject(cep)
	if len(cep) == 8 && typecep == "string" {
		logger.InfoContext(ctx, cep,  )
		// https://opentelemetry.io/docs/languages/go/getting-started/
		body, _ := json.Marshal(map[string]string{
			"cep": cep,
		})
		payload := bytes.NewBuffer(body)

		req, err := http.Post("http://localhost:8080/cep", "application/json", payload)
		if err != nil {
			fmt.Printf("error in post operation")
			if req.StatusCode == 422 {
				fmt.Printf("invalid zipcode")
			}
		}

		defer req.Body.Close()
		body, err = io.ReadAll(req.Body)
		if err != nil {
			fmt.Printf("%s", err)
		}
		defer req.Body.Close()

		fmt.Printf("%s", body)
	}
}
