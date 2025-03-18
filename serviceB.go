package main

import (
	"encoding/json"

	"fmt"
	"io"

	"net/http"
	neturl "net/url"
	"os"




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


func cepHandler(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")

	// request para pegar localidade
	if len(cep) == 8 {
		req, err := http.Get("http://viacep.com.br/ws/" + cep + "/json/")
		if err != nil {
			fmt.Println("error in requisition via CEP")
		}
		if req.StatusCode != 200 {
			if req.StatusCode == 400 {
				err = fmt.Errorf("bad request")
				fmt.Println(err)
				os.Exit(1)
			}
			if req.StatusCode == 404 {
				err = fmt.Errorf("can not find zipcode")
				fmt.Println(err)
				os.Exit(1)
			}
			if req.StatusCode == 422 {
				err = fmt.Errorf("invalid  zipcode")
				fmt.Println(err)
				os.Exit(1)
			}
		}

		defer req.Body.Close()

		res, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Println("error in reading the body via CEP")
		}
		var data ViaCEP
		fmt.Println(data)
		err = json.Unmarshal(res, &data)
		if err != nil {
			fmt.Println("error in unmarshal via CEP")
		}
		local := data.Localidade

		url := "http://api.weatherapi.com/v1/current.json?key=18525c8de5ac479f994185201250303&q=" + neturl.QueryEscape(local) + "&aqi=no"

		// novo request para pegar a temperatura
		req2, err2 := http.Get(url)
		if err2 != nil {
			fmt.Println("error in requisition via WeatherAPI")
		}
		defer req2.Body.Close()

		res2, err2 := io.ReadAll(req2.Body)
		if err2 != nil {
			fmt.Println("error in reading the body via WeatherAPI")
		}
		var data2 Weather
		err2 = json.Unmarshal(res2, &data2)
		if err2 != nil {
			fmt.Println("error in unmarshal via WeatherAPI")
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
	} else {
		return
	}
}
