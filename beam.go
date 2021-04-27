// a script for getting normal and differentially private counts of the top viewed
// pages for various wikipedia language projects. DP counts vary with configurations
// of epsilon and delta.

// should be run after init_db and before clean_db

package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/htried/wiki-diff-privacy/wdp"

	"github.com/apache/beam/sdks/go/pkg/beam"
	"github.com/apache/beam/sdks/go/pkg/beam/io/databaseio"
	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
)

// initialize functions and types to be referenced in beam
func init() {
	beam.RegisterType(reflect.TypeOf((*pageView)(nil)))
	beam.RegisterType(reflect.TypeOf((*wdp.TableRow)(nil)))
	beam.RegisterFunction(extractPage)
}

// struct to represent 1 synthetic pageview
type pageView struct {
	ID   string // a synthetic "unique id" for the page view
	Name string // the name of the page visited
}

// the function that runs when `go run beam.go` is typed into the command line
// it iterates through each language code and runs the pipeline on it
func main() {
	start := time.Now()
	for _, lang := range wdp.LanguageCodes {
		err := processLanguage(lang)
		if err != nil {
			log.Printf("Error processing language %s\n", err)
			return
		}
	}
	log.Printf("Time to count all languages: %v seconds\n", time.Now().Sub(start).Seconds())
}

// create an output db for an individual language
func processLanguage(lang string) error {
	start := time.Now()

	// make a map that goes from epsilon/delta parameters --> output PCollections
	dpMap := make(map[string]beam.PCollection)

	// get the DSN
	// NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
	dsn, err := wdp.DSN("wdp") // LOCAL & CLOUD VPS
	// dsn, err := wdp.DSN("s54717__wdp_p") // TOOLFORGE
	if err != nil {
		log.Printf("Error %s when getting dbname\n", err)
		return err
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
	for _, eps := range wdp.Epsilons {
		for _, del := range wdp.Deltas {
			// cast them to a string
			key := strconv.FormatFloat(eps, 'f', -1, 64) + "|" + strconv.FormatFloat(del, 'f', -1, 64)

			// map that string to the PCollection you get when you do a DP page count
			dpMap[key] = privateCountPageViews(s, pvs, eps, del)
		}
	}

	// get the name of the table from the input language and date
	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
	lang = strings.ReplaceAll(lang, "-", "_")

	tbl_name := fmt.Sprintf("output_%s_%s", lang, yesterday)

	// write normal count to db
	databaseio.Write(s, "mysql", dsn, tbl_name, []string{}, normalCount)

	// for each (epsilon, delta) tuple
	for _, privCount := range dpMap {
		// write that DP count to db
		databaseio.Write(s, "mysql", dsn, tbl_name, []string{}, privCount)
	}

	// execute the pipeline
	_, err = direct.Execute(context.Background(), p)
	if err != nil {
		log.Print("execution of pipeline failed: %v", err)
		return err
	}

	log.Printf("Time to do all counts of language %s: %v seconds\n", lang, time.Now().Sub(start).Seconds())

	return nil
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

// a function that uses Beam to count the raw number of pageviews for each of
// the top 50 pages viewed in a given language project
func countPageViews(s beam.Scope, col beam.PCollection) beam.PCollection {
	s = s.Scope("countPageViews")

	// extract the page names from the input PCollection of pageviews from db
	// yields PCollection<string>
	pageviews := beam.ParDo(s, extractPage, col)

	// count the number of times each page shows up in pageviews
	// yields PCollection<string,int>
	counted := stats.Count(s, pageviews)

	// create constants for "epsilon" and "delta". these are both -1 to signify
	// that this is the normal count.
	eps := beam.Create(s, float64(-1))
	del := beam.Create(s, float64(-1))

	// using eps and del as side inputs, emit a set of wdp.TableRows
	// yields PCollection<wdp.TableRow>
	out := beam.ParDo(s, func(k string, v int, epsilon, delta float64, emit func(out wdp.TableRow)) {
		emit(wdp.TableRow{
			Name:    k,
			Views:   v,
			Epsilon: epsilon,
			Delta:   delta,
		})
	}, counted, beam.SideInput{Input: eps}, beam.SideInput{Input: del})
	return out
}

// a function that uses Privacy on Beam to count the number of pageviews for each
// of the top 50 pages viewed in a given language project in a differentially-
// private fashion based on epsilon and delta
func privateCountPageViews(s beam.Scope, col beam.PCollection, epsilon, delta float64) beam.PCollection {
	s = s.Scope("privateCountPageViews")

	// create the privacy spec and create a PrivatePCollection
	// yields PrivatePCollection<PageView>
	spec := pbeam.NewPrivacySpec(epsilon, delta)
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "ID")

	// extract the page names from the PrivatePCollection of pageviews
	// yields PrivatePCollection<string>
	pageviews := pbeam.ParDo(s, extractPage, pCol)

	// privately count the number of times each page shows up in pageviews
	// yields PrivatePCollection<string,int>
	counted := pbeam.Count(s, pageviews, pbeam.CountParams{ // defaults to Laplace noise
		MaxPartitionsContributed: 1, // In the scheme I've constructed, each visitor visits 1x per day (on user-level privacy, this would go to 5-10)
		MaxValue:                 1, // And they can visit a maximum of 1 page
	})

	// create constants for epsilon and delta
	eps := beam.Create(s, epsilon)
	del := beam.Create(s, delta)

	// using eps and del as side inputs, emit a set of wdp.TableRows
	// yields PCollection<wdp.TableRow>
	out := beam.ParDo(s, func(k string, v int64, epsilon, delta float64, emit func(out wdp.TableRow)) {
		emit(wdp.TableRow{
			Name:    k,
			Views:   int(v),
			Epsilon: epsilon,
			Delta:   delta,
		})
	}, counted, beam.SideInput{Input: eps}, beam.SideInput{Input: del})

	return out
}

// a simple wrapper DoFn for extracting a page name from a pageView
func extractPage(p pageView) string {
	return p.Name
}
