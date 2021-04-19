package main

import (
    "fmt"
    "net/http"
    "log"
    "html/template"
  	"strings"
  	"time"
  	"os"
  	"reflect"
  	"context"
  	"regexp"
  	"math"
  	"encoding/json"


	"github.com/apache/beam/sdks/go/pkg/beam"

	// The following import is required for accessing local files.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
	"github.com/htried/wiki-diff-privacy/wdp"
	
	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
)

// parameters to send to the client for display
type outParams struct {
	Lang 				string	`json:"lang"`
	Eps 				float64 `json:"eps"`
	Sensitivity			int 	`json:"sensitivity"` // TODO: change to delta
	QualEps 			float64 `json:"qual-eps"`		
	Alpha 				float64 `json:"alpha"`
	PropWithin 			float64 `json:"prop-within"`
	AggregateThreshold	float64 `json:"aggregate-threshold"`
}

// struct that sends back all outbound data
type output struct {
	Params		outParams 					`json:"params"`
	Results 	map[string]map[string]int 	`json:"results"`
}



// function to load the homepage of the site
func Index(w http.ResponseWriter, r *http.Request) {
	// validate the passed-in arguments
	vars, err := wdp.ValidateApiArgs(r)
	if err != nil {
		log.Print("error validating API arguments: ", err)
	}

	// parse the template at index.html
	t, err := template.ParseFiles("templates/index.html")
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
		log.Print("error validating API arguments: ", err)
	}

	// TODO: update this to get data from db
	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
	err = wdp.RemoveOldContents(yesterday, "data/")
	if err != nil {
		log.Print("error removing contents of data folder: ", err)
	}

	// TODO: remove this
	fname := fmt.Sprintf("./data/synthetic_data_%s_%s.csv", vars.Lang, yesterday)
	outname := fmt.Sprintf("./data/output_%s_%s.csv", vars.Lang, yesterday)
	outnameDP := fmt.Sprintf("./data/dp_output_%s_%s.csv", vars.Lang, yesterday)

	// TODO: update this to get data from db
	_, err = os.Stat(fname)
	if os.IsNotExist(err) {
		err = wdp.InitializeSyntheticData(yesterday, vars.Lang)
		if err != nil {
			log.Print("error initializing synthetic data from yesterday: ", err)
		}
	} else if err != nil {
		log.Print("error stat-ing file: ", err)
		return
	}

	// TODO: udpate this to get data from db
	results, err := wdp.CreateOutputStruct(outname, outnameDP, vars)
	if err != nil {
		log.Print("creation of output struct failed: %v", err)
	}

	// create outward facing parameters
	var params outParams
	params.Lang = vars.Lang
	params.Eps = vars.Epsilon
	params.Sensitivity = vars.Sensitivity
	params.QualEps = wdp.QualEps(vars.Epsilon, 0.5)
	params.Alpha = vars.Alpha
	params.PropWithin = vars.PropWithin
	params.AggregateThreshold = wdp.AggregationThreshold(vars.Sensitivity, vars.Epsilon, vars.Alpha, vars.PropWithin)

	// put outward facing parameters and results into one struct
	var out output
	out.Params = params
	out.Results = results

	// send the struct back as a json file
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}


// bind functions to paths and start listener
func main() {
	// undo at the end
    http.HandleFunc("/", Index)
    http.HandleFunc("/api/v1/pageviews", PageViews)
    // http.HandleFunc("/", PageViews)
    log.Fatal(http.ListenAndServe(":5000", nil))
}
