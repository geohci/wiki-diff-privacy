package wdp

import (
	"fmt"
    "net/http"
    "io"
    "encoding/json"
  	"strconv"
  	"time"
  	"os"
  	"encoding/csv"
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

func CreateOutputStruct(fname, fnameDP string, vars PageVars) (map[string]map[string]int, error) {
	output := make(map[string]map[string]int)

	articles, err := readAndSort(fname)
	if err != nil {
		return output, err
	}

	articlesDP, err := readAndSort(fname)
	if err != nil {
		return output, err
	}

	for i, art := range articles {
		articleEntry := make(map[string]int)
		output[art.Name] = articleEntry
		output[art.Name]["gt-rank"] = i
		output[art.Name]["gt-views"] = art.Views
	}

	for i, art := range articlesDP {
		output[art.Name]["dp-rank"] = i
		output[art.Name]["dp-views"] = art.Views
		output[art.Name]["do-aggregate"] = DoAggregate(art.Views, vars.Sensitivity, vars.Epsilon, vars.Alpha, vars.PropWithin)
	}

	return output, nil
}

func readAndSort(fname string) ([]Article, error) {
	var articles []Article

	f, err := os.Open(fname)
	defer f.Close()
	if err != nil {
		return articles, err
	}

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return articles, err
	}

	for _, rec := range records {
		v, err := strconv.Atoi(rec[1])
		if err != nil {
			return articles, err
		}
		var art Article
		art.Name = rec[0]
		art.Views = v
		articles = append(articles, art)
		
	}

	sort.SliceStable(articles, func(i, j int) bool {
		return articles[i].Views > articles[j].Views
	})
	// log.Print(articles)

	return articles, nil
}

// enable CORS headers for the API
func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
