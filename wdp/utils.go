// various functions for making API calls, creating output structs, accessing
// the DB, etc.

package wdp

import (
	"fmt"
    "net/http"
    "io"
    "encoding/json"
  	"time"
  	"sort"
)

type Article struct {
	Name	string 	`json:"article"`
	Views 	int 	`json:"views"`
	Rank 	int 	`json:"rank"`
}

type Project struct {
	Project 	string 		`json:"project"`
	Access		string 		`json:"access"`
	Year 		string 		`json:"year"`
	Month		string 		`json:"month"`
	Day 		string 		`json:"day"`
	Articles 	[]Article 	`json:"articles"`
}

type ApiResponse struct {
	Items 	[]Project 	`json:"items"`
}

// struct to represent a row of the output table
type TableRow struct {
	Name 	string 		// name of the page visited
	Views 	int 		// number of views for the day
	Epsilon float64 	// epsilon used for calculating the number of views (-1 is none)
	Delta 	float64 	// delta used for calculating the number of views (-1 is none)
}

// get the top 50 highest-performing articles for a given language lang from
// yesterday or the day before
func GetGroundTruth(lang string) ([50]Article, error) {

	// create query string
	var yesterday = time.Now().AddDate(0, 0, -1)
	var apiString = fmt.Sprintf("https://wikimedia.org/api/rest_v1/metrics/pageviews/top/%s.wikipedia.org/all-access/%s", lang, yesterday.Format("2006/01/02"))

	// do get query for yesterday
	resp, err := http.Get(apiString)
	if err != nil {
		return [50]Article{}, err
	}

	// if there's an unsatisfactory response status code, try the day before yesterday
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var dayBeforeYesterday = time.Now().AddDate(0, 0, -2)
		apiString = fmt.Sprintf("https://wikimedia.org/api/rest_v1/metrics/pageviews/top/%s.wikipedia.org/all-access/%s", lang, dayBeforeYesterday.Format("2006/01/02"))
		resp, err = http.Get(apiString)
		if err != nil {
			return [50]Article{}, err
		} 
	}

	defer resp.Body.Close()

	// read the response body and unmarshal it into a go struct
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return [50]Article{}, err
	} 
	var apiResp ApiResponse
	json.Unmarshal([]byte(body), &apiResp)

	// from the response return the top 50 articles
	var topFiftyArticles [50]Article
	if len(apiResp.Items) == 0 {
		return [50]Article{}, err
	}

	for i, article := range apiResp.Items[0].Articles {
		if i < 50 {
			topFiftyArticles[i] = article
		} else {
			break
		}
	}

	return topFiftyArticles, nil
}

// create an output structure to send over JSON that will play nicely with the html template
func CreateOutputStruct(normalCount, dpCount []TableRow, vars PageVars) map[string]map[string]int {
	// create output map
	output := make(map[string]map[string]int)

	// sort the normalCount and dpCount lists by number of views
	sort.SliceStable(normalCount, func(i, j int) bool {
		return normalCount[i].Views > normalCount[j].Views
	})
	sort.SliceStable(dpCount, func(i, j int) bool {
		return dpCount[i].Views > dpCount[j].Views
	})

	// for each normal article
	for i, art := range normalCount {
		// create a map and input its rank and number of views
		articleEntry := make(map[string]int)
		output[art.Name] = articleEntry
		output[art.Name]["gt-rank"] = i + 1
		output[art.Name]["gt-views"] = art.Views
		
		// add all these -1 entries to the map, because some counts may be too
		// small to show up and we need a signifier of that
		output[art.Name]["dp-rank"] = -1
		output[art.Name]["dp-views"] = -1
		output[art.Name]["do-aggregate"] = -1
	}

	// log.Print(output)

	// for each DP-altered article
	for i, art := range dpCount {
		// log.Print(art)
		// add the DP rank, DP views, and whether or not you should aggregate
		output[art.Name]["dp-rank"] = i + 1
		output[art.Name]["dp-views"] = art.Views
		output[art.Name]["do-aggregate"] = DoAggregate(art.Views, vars.Sensitivity, vars.Epsilon, vars.Alpha, vars.PropWithin)
	}

	return output
}


// enable CORS headers for the API
func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
