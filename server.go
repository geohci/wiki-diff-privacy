package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"

	"github.com/htried/wiki-diff-privacy/wdp"
)

// parameters to send to the client for display
type outParams struct {
	Lang               string  `json:"lang"`
	Eps                float64 `json:"eps"`
	Delta              float64 `json:"delta"`
	Sensitivity        int     `json:"sensitivity"`
	QualEps            float64 `json:"qual-eps"`
	Alpha              float64 `json:"alpha"`
	PropWithin         float64 `json:"prop-within"`
	AggregateThreshold float64 `json:"aggregate-threshold"`
}

// struct that sends back all outbound data
type output struct {
	Params  outParams                 `json:"params"`
	Results map[string]map[string]int `json:"results"`
}

// global variables for db and error
var db *sql.DB
var err error

// function to load the homepage of the site
func Index(w http.ResponseWriter, r *http.Request) {
	// validate the passed-in arguments
	vars, err := wdp.ValidateApiArgs(r)
	if err != nil {
		log.Print("error validating API arguments: ", err)
	}

	// parse the template at index.html
	// NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON CLOUD VPS VS LOCALLY
	// t, err := template.ParseFiles("templates/index.html") // LOCAL
	t, err := template.ParseFiles("/etc/diff-privacy-beam/index.html") // CLOUD VPS
	if err != nil {
		log.Print("error parsing template index_go.html: ", err)
	}

	// execute the template to serve it back to the client
	err = t.Execute(w, vars)
	if err != nil {
		log.Print("error executing template index_go.html: ", err)
	}
}

// function to call the API
func PageViews(w http.ResponseWriter, r *http.Request) {

	// enable outside API requests
	wdp.EnableCors(&w)

	// validate input API args
	vars, err := wdp.ValidateApiArgs(r)
	if err != nil {
		log.Printf("error %v validating API arguments\n", err)
		return
	}

	// query the database to get normalCount and dpCount
	normalCount, dpCount, err := wdp.Query(db, vars.Lang, vars.Epsilon, vars.Delta)
	if err != nil {
		log.Printf("error %v querying database\n", err)
		return
	}

	// feed those into a util function to format them correctly
	results := wdp.CreateOutputStruct(normalCount, dpCount, vars)

	// create outward facing parameters
	var params = outParams{
		Lang: 				vars.Lang,
		Eps: 				vars.Epsilon,
		Sensitivity: 		vars.Sensitivity,
		QualEps: 			wdp.QualEps(vars.Epsilon, 0.5),
		Alpha: 				vars.Alpha,
		PropWithin: 		vars.PropWithin,
		AggregateThreshold: wdp.AggregationThreshold(vars.Sensitivity, vars.Epsilon, vars.Alpha, vars.PropWithin),
	}

	// put outward facing parameters and results into one struct
	var out = output{
		Params: 	params,
		Results:	results,
	}

	// send the struct back as a json file
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// get DB connection, bind functions to paths, and start listener
func main() {
	// connect to the DB
	db, err = wdp.DBConnection()
	if err != nil {
		panic(err.Error())
	}

	http.HandleFunc("/", Index)
	http.HandleFunc("/api/v1/pageviews", PageViews)

	log.Fatal(http.ListenAndServe(":8000", nil))
}
