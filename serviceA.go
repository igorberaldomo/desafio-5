package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	

)

func main() {
	http.HandleFunc("/", cepValidatorHandler)
	http.ListenAndServe(":8080", nil)
}
func typeofObject(variable interface{}) string {
	return fmt.Sprintf("%T", variable)
 }
 
func cepValidatorHandler(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")
	typecep:= typeofObject(cep)
	if len(cep) == 8 && typecep == "string" {

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
