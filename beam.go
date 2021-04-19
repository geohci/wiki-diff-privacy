// 

package main

import (
	"fmt"
	"reflect"
	"math"
	"time"
	"strings"
	"strconv"
	"log"
	"context"

	"github.com/htried/wiki-diff-privacy/wdp"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"github.com/apache/beam/sdks/go/pkg/beam/io/databaseio"
)

// initialize functions and types to be referenced in beam
func init() {
	beam.RegisterType(reflect.TypeOf((*pageView)(nil)))
	beam.RegisterType(reflect.TypeOf((*output)(nil)))
	beam.RegisterFunction(extractPage)
}


// various configurations of epsilon and delta to compute the view count per page with
var epsilon = []float64{0.1, 0.5, 1, 5}
var delta = []float64{math.Pow10(-4), math.Pow10(-2), 0.1, 0.5}

// struct to represent 1 synthetic pageview
type pageView struct {
	ID 		string 		// a synthetic "unique id" for the page view
	Name 	string 		// the name of the page visited
}

// struct to represent a row of output
type output struct {
	Name 	string 		// name of the page visited
	Views 	int 		// number of views for the day
	Epsilon float64 	// epsilon used for calculating the number of views (-1 is none)
	Delta 	float64 	// delta used for calculating the number of views (-1 is none)
}

// the function that runs when `go run beam.go` is typed into the command line
// it iterates through each language code and runs the pipeline on it
func main() {
	log.Printf("len: %v\n", len(wdp.LanguageCodes))
	for _, lang := range wdp.LanguageCodes {
		err := processLanguage(lang)
		if err != nil {
			log.Printf("Error processing language %s\n", err)
			return
		}
	}
}

// create an output db for an individual language
func processLanguage(lang string) error {

	// make a map that goes from epsilon/delta parameters --> output PCollections
	dpMap := make(map[string]beam.PCollection)

	// get the DSN
	dsn, err := wdp.DSN("wdp")
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return err
    }

    // set up a table in the db to write to
    tbl_name, err := tableSetUp(lang)
    if err != nil {
    	log.Printf("Error %s when setting up tables\n", err)
    }

    // initialize the Beam pipeline
	beam.Init()
	p := beam.NewPipeline()
	s := p.Root()

	// read in the input from the database
	pvs := readInput(s, dsn, lang)

	// do the normal Beam count
	normalCount := countPageViews(s, pvs)

	// for each (epsilon, delta) tuple
	for _, eps := range epsilon {
		for _, del := range delta {
			// cast them to a string
			key := strconv.FormatFloat(eps, 'f', -1, 64) + "|" + strconv.FormatFloat(del, 'f', -1, 64)

			// map that string to the PCollection you get when you do a DP page count
			dpMap[key] = privateCountPageViews(s, pvs, eps, del)
		}
	}

	// write normal count to db
	databaseio.Write(s, "mysql", dsn, tbl_name, []string{}, normalCount)

	// for each (epsilon, delta) tuple
	for _, privCount := range dpMap {
		// write that DP count to db
		databaseio.Write(s, "mysql", dsn, tbl_name, []string{}, privCount)
	}

	// Execute pipeline.
	_, err = direct.Execute(context.Background(), p)
	if err != nil {
		log.Print("execution of pipeline failed: %v", err)
		return err
	}

	return nil
}

func tableSetUp(lang string) (string, error) {
	db, err := wdp.DBConnection()
	if err != nil {
    	log.Printf("Error %s when connecting to db\n", err)
    	return "", err
    }
    defer db.Close()

    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")

    // create table for output
    tbl_name := fmt.Sprintf("output_%s_%s", lang, yesterday)
    err = wdp.CreateTable(db, tbl_name)
    if err != nil {
    	log.Printf("Error %s when creating new table\n", err)
    	return "", err
    }

    return tbl_name, nil
}


// readInput reads from a database detailing page views in the form
// of "id, name" and returns a PCollection of pageView structs.
func readInput(s beam.Scope, dsn, lang string) beam.PCollection {
	s = s.Scope("readInput")

	// get the name of the table from the input language and date
    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
    lang = strings.ReplaceAll(lang, "-", "_")
    tbl_name := fmt.Sprintf("data_%s_%s", lang, yesterday)

    // read from the database into a PCollection of pageView structs
	return databaseio.Read(s, "mysql", dsn, tbl_name, reflect.TypeOf(pageView{}))
}


func countPageViews(s beam.Scope, col beam.PCollection) beam.PCollection {
	s = s.Scope("countPageViews")
	pageviews := beam.ParDo(s, extractPage, col)

	counted := stats.Count(s, pageviews)


	eps := beam.Create(s, float64(-1))
	del := beam.Create(s, float64(-1))
	out := beam.ParDo(s, func(k string, v int, epsilon, delta float64, emit func(out output)) {
		emit(output{
			Name: k,
			Views: v,
			Epsilon: epsilon,
			Delta: delta,
		})
	}, counted, beam.SideInput{Input: eps}, beam.SideInput{Input: del})
	return out
}

func privateCountPageViews(s beam.Scope, col beam.PCollection, epsilon, delta float64) beam.PCollection {
	s = s.Scope("privateCountPageViews")

	spec := pbeam.NewPrivacySpec(epsilon, delta)
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "ID")

	pageviews := pbeam.ParDo(s, extractPage, pCol)
	counted := pbeam.Count(s, pageviews, pbeam.CountParams{
		MaxPartitionsContributed:	1, 					// In the scheme I've constructed, each visitor visits 1x per day
		MaxValue: 					1, 					// And they can visit a maximum of 1 page
		// NoiseKind: 					pbeam.LaplaceNoise,	// We're using Laplace noise for this count
	})
	eps := beam.Create(s, epsilon)
	del := beam.Create(s, delta)
	out := beam.ParDo(s, func(k string, v int64, epsilon, delta float64, emit func(out output)) {
		emit(output{
			Name: k,
			Views: int(v),
			Epsilon: epsilon,
			Delta: delta,
		})
	}, counted, beam.SideInput{Input: eps}, beam.SideInput{Input: del})

	return out
}

func extractPage(p pageView) string {
	return p.Name
}
