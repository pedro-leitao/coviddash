//
// This is a very simple dashboard server which presents basic stats on COVID results for any given country.
// Results are retrieved from https://api.covid19api.com/ and are plotted as various summary charts.
//
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenjiandongx/go-echarts/charts"
	"github.com/gorilla/mux"
)

const RetrievalError string = "Could not retrieve for country code"
const ParsingError string = "Could not parse JSON for country code"

type dayOneResults []struct {
	Country     string    `json:"Country"`
	CountryCode string    `json:"CountryCode"`
	Province    string    `json:"Province"`
	City        string    `json:"City"`
	CityCode    string    `json:"CityCode"`
	Lat         string    `json:"Lat"`
	Lon         string    `json:"Lon"`
	Confirmed   int       `json:"Confirmed"`
	Deaths      int       `json:"Deaths"`
	Recovered   int       `json:"Recovered"`
	Active      int       `json:"Active"`
	Date        time.Time `json:"Date"`
}

// Retrieves a dataset for a given country
func retrieveDayOneCountryStats(countryCode string) (dayOneResults, error) {
	resp, err := http.Get("https://api.covid19api.com/total/dayone/country/" + countryCode)

	if err != nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf(RetrievalError+" %v (%v, %v)", countryCode, resp.Status, err)
	}
	defer resp.Body.Close()

	results := dayOneResults{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(RetrievalError+" %v (%v, %v)", countryCode, resp.Status, err)
	}

	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, fmt.Errorf(ParsingError+" %v (%v, %v)", countryCode, resp.Status, err)
	}

	return results, nil

}

func singleCountry(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	results, err := retrieveDayOneCountryStats(vars["code"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, err.Error())
	} else {
		w.WriteHeader(http.StatusOK)
		w.Header().Add("content-type", "application/html")
		renderCountryChart(results, vars["code"], w)
	}
}

func renderCountryChart(results dayOneResults, countryCode string, w io.Writer) error {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.TitleOpts{Title: "COVID cases for " + strings.ToUpper(results[0].Country)},
		charts.ToolboxOpts{Show: true},
	)
	xvalues := []string{}
	confirmed := []int{}
	deaths := []int{}
	// recovered := []int{}
	// active := []int{}
	for _, v := range results {
		xvalues = append(xvalues, v.Date.Format("Jan 02"))
		confirmed = append(confirmed, v.Confirmed)
		deaths = append(deaths, v.Deaths)
		// recovered = append(recovered, v.Recovered)
		// active = append(active, v.Active)
	}
	line.AddXAxis(xvalues).
		AddYAxis("Confirmed cases", confirmed, charts.LabelTextOpts{Show: false, Position: "bottom"}).
		AddYAxis("Deaths", deaths, charts.LabelTextOpts{Show: false, Position: "bottom"})
	line.SetSeriesOptions(
		charts.MLNameTypeItem{Name: "Avg", Type: "average"},
		charts.LineOpts{Smooth: true},
		charts.MLStyleOpts{Label: charts.LabelTextOpts{Show: true, Formatter: "{a}: {b}"}},
	)
	return line.Render(w)
}

func main() {
	port := flag.Int("port", 4040, "port number server should run on")
	flag.Parse()

	router := mux.NewRouter()
	router.HandleFunc("/country/{code}", singleCountry)

	srv := &http.Server{
		Handler: router,
		Addr:    ":" + strconv.Itoa(*port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
