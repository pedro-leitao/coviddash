//
// This is a very simple dashboard server which presents basic stats on COVID results for any given country.
// Results are retrieved from https://api.covid19api.com/ and are plotted as various summary charts.
//
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenjiandongx/go-echarts/charts"
	"github.com/gorilla/mux"
)

// RetrievalError is the template text for a retrieval error
const RetrievalError string = "Could not retrieve for country code"

// ParsingError is the template text for a parsing error
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

func multipleCountries(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	allCountries := []string{}
	deathRates := []int{}
	daysToFirstDeath := []int{}
	totalCases := make(map[string]interface{})
	totalDeaths := make(map[string]interface{})

	countryList := strings.Split(vars["countries"], " ")
	page := charts.NewPage(charts.RouterOpts{})
	page.PageTitle = "COVID Dashboard for " + time.Now().Format(time.RFC822)
	defer req.Body.Close()

	log.Printf("Processing new request from %v\n", req.RemoteAddr)
	w.Header().Add("content-type", "text/html")

	for _, country := range countryList {
		results, err := retrieveDayOneCountryStats(country)
		if err != nil {
			log.Printf("%v", err)
			continue
		} else {
			if len(results) == 0 {
				continue
			}
			timeEvolutionChart, dr, dtfd, tc, td, _ := getCountryChart(results, country)
			allCountries = append(allCountries, strings.ToUpper(country))
			deathRates = append(deathRates, dr)
			daysToFirstDeath = append(daysToFirstDeath, dtfd)
			totalCases[strings.ToUpper(country)] = tc
			totalDeaths[strings.ToUpper(country)] = td
			page.Add(timeEvolutionChart)
			log.Printf("Added chart for %v in response\n", country)
		}
	}

	deathRatesByCountryChart := charts.NewEffectScatter()
	deathRatesByCountryChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Death Rates/Country"},
		charts.InitOpts{Width: "1280"},
	)
	deathRatesByCountryChart.AddXAxis(allCountries).
		AddYAxis("Death Rate", deathRates, charts.RippleEffectOpts{Period: 4, Scale: 6, BrushType: "stroke"}).
		AddYAxis("Days to First Death", daysToFirstDeath, charts.RippleEffectOpts{Period: 4, Scale: 3, BrushType: "fill"})
	deathRatesByCountryChart.SetSeriesOptions(charts.LabelTextOpts{Show: true, Position: "right"})

	casesByCountryChart := charts.NewPie()
	casesByCountryChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Confirmed Cases/Deaths"},
		charts.InitOpts{Width: "1280"},
	)
	casesByCountryChart.Add("cases", totalCases,
		charts.LabelTextOpts{Show: true, Formatter: "{b}: {c}"},
		charts.PieOpts{Radius: []string{"20%", "65%"}, RoseType: "area", Center: []string{"25%", "50%"}},
	)
	casesByCountryChart.Add("deaths", totalDeaths,
		charts.LabelTextOpts{Show: true, Formatter: "{b}: {c}"},
		charts.PieOpts{Radius: []string{"20%", "65%"}, RoseType: "area", Center: []string{"75%", "50%"}},
	)

	page.Add(deathRatesByCountryChart)
	page.Add(casesByCountryChart)
	page.Render(w)
	log.Printf("Finished serving request for %v\n", req.RemoteAddr)
}

func getCountryChart(results dayOneResults, countryCode string) (*charts.Line, int, int, int, int, string) {
	line := charts.NewLine()

	xvalues := []string{}
	increaseCases := []int{}
	increaseDeaths := []int{}
	increaseDeaths = append(increaseDeaths, 0)
	previousCases := 0
	previousDeaths := 0
	deathRate := 0
	daysToFirstDeath := 0
	for i, v := range results {
		xvalues = append(xvalues, v.Date.Format("Jan 02"))
		deathRate = (int)((float64)(v.Deaths) / (float64)(v.Confirmed) * 100.0)
		increaseCases = append(increaseCases, v.Confirmed-previousCases)
		increaseDeaths = append(increaseDeaths, v.Deaths-previousDeaths)
		previousCases = v.Confirmed
		previousDeaths = v.Deaths
		if daysToFirstDeath == 0 && v.Deaths > 0 {
			daysToFirstDeath = i
		}
	}

	line.SetGlobalOptions(
		charts.InitOpts{PageTitle: "COVID Dashboard for " + time.Now().Format(time.RFC822), Width: "1280"},
		charts.TitleOpts{
			Title:         "COVID cases for " + strings.ToUpper(results[0].Country),
			Subtitle:      "Accumulated ☠ rate of " + strconv.Itoa(deathRate) + " in 100",
			SubtitleStyle: charts.TextStyleOpts{Color: "#909090", FontSize: 16},
		},
		charts.ToolboxOpts{Show: false},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
		charts.XAxisOpts{SplitLine: charts.SplitLineOpts{Show: true}},
		charts.YAxisOpts{SplitLine: charts.SplitLineOpts{Show: true}},
	)
	line.AddXAxis(xvalues).
		AddYAxis("Δ in deaths", increaseDeaths,
			charts.MPNameTypeItem{Type: "max", Name: "Maximum"},
			charts.MPNameTypeItem{Type: "average", Name: "Average"},
			charts.MPStyleOpts{Label: charts.LabelTextOpts{Show: true}},
			charts.AreaStyleOpts{Opacity: 0.2},
			charts.LineOpts{Smooth: true},
		).
		AddYAxis("Δ in confirmed cases", increaseCases,
			charts.MPNameTypeItem{Type: "max", Name: "Maximum"},
			charts.MPNameTypeItem{Type: "average", Name: "Average"},
			charts.MPStyleOpts{Label: charts.LabelTextOpts{Show: true}},
			charts.LineOpts{Smooth: true},
		).
		SetSeriesOptions(
			charts.MLStyleOpts{Label: charts.LabelTextOpts{Show: true, Formatter: "Δ {b}"}},
		)

	return line, deathRate, daysToFirstDeath, previousCases, previousDeaths, results[0].Country
}

func main() {
	port := flag.Int("port", 4040, "port number server should run on")
	flag.Parse()

	router := mux.NewRouter()
	router.HandleFunc("/countries", multipleCountries).Queries("countries", "{countries}")

	srv := &http.Server{
		Handler:      router,
		Addr:         ":" + strconv.Itoa(*port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}
