// a script for getting normal and differentially private counts of the top viewed
// pages for various wikipedia language projects. DP counts vary with configurations
// of epsilon and delta.

// should be run after init_db and before clean_db

package main

import (
	"context"
	"log"
	"reflect"
	"strconv"
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
	beam.RegisterFunction(filterLang)
}

// struct to represent 1 synthetic pageview
type pageView struct {
	PV_ID   	string // a synthetic unique id for the pageview
	User_ID 	string // a synthetic user id for the pageview
	Day 		string // date of the pageview
	Lang 		string // wikipedia project the pageview comes from
	Name 		string // the name of the page visited
}

type dbParams struct {
	Lang 		string // language of an aggregation
	Day 		string // date of an aggregation
	Kind 		string // kind of aggregation (either 'pv' or 'user')
	Epsilon 	float64
	Delta 		float64
}

// the function that runs when `go run beam.go` is typed into the command line
// it iterates through each language code and runs the pipeline on it
func main() {
	start := time.Now()

	// get the DSN
	// NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
	dsn, err := wdp.DSN("wdp") // LOCAL & CLOUD VPS
	// dsn, err := wdp.DSN("s54717__wdp_p") // TOOLFORGE
	if err != nil {
		log.Printf("Error %s when getting dbname\n", err)
		return
	}

	// initialize the Beam pipeline
	beam.Init()
	p := beam.NewPipeline()
	s := p.Root()

	// read in the input from the database
	pvs := readInput(s, dsn)

	// for each language
	for _, lang := range wdp.LanguageCodes {
		// make a map that goes from epsilon/delta parameters --> output PCollections
		dpMap := make(map[string]beam.PCollection)

		// filter the initial db to be just that language
		langBeam := beam.Create(s, lang)
		filtered := beam.ParDo(s, filterLang, pvs, beam.SideInput{Input: langBeam})

		// set up params that we will write to the output db
		normalParams := dbParams{
			Lang: 		lang,
			Day: 		time.Now().AddDate(0, 0, -1).Format("2006-01-02"), // yesterday
			Kind: 		"pv",
			Epsilon: 	float64(-1),
			Delta: 		float64(-1),
		}

		// do the normal Beam count, passing in params
		normalCount := countPageViews(s, filtered, normalParams)

		// for each (epsilon, delta) tuple
		for _, eps := range wdp.Epsilons {
			for _, del := range wdp.Deltas {
				// cast them to a string as a key
				key := strconv.FormatFloat(eps, 'f', -1, 64) + "|" + strconv.FormatFloat(del, 'f', -1, 64)

				// set up params that we will write to the output db
				dpParams := dbParams{
					Lang: 		lang,
					Day: 		time.Now().AddDate(0, 0, -1).Format("2006-01-02"), // yesterday
					Kind: 		"pv",
					Epsilon: 	eps,
					Delta: 		del,
				}

				// map that string to the PCollection you get when you do a DP page count, passing in params
				dpMap[key] = privateCountPageViews(s, filtered, dpParams)
			}
		}

		// write normal count to db
		databaseio.Write(s, "mysql", dsn, "output", []string{}, normalCount)

		// for each (epsilon, delta) tuple
		for _, privCount := range dpMap {
			// write that DP count to db
			databaseio.Write(s, "mysql", dsn, "output", []string{}, privCount)
		}
	}

	// execute the pipeline
	_, err = direct.Execute(context.Background(), p)
	if err != nil {
		log.Print("execution of pipeline failed: %v", err)
		return
	}

	log.Printf("Time to count all languages: %v seconds\n", time.Now().Sub(start).Seconds())
}

// readInput reads from a database detailing page views in the form
// of "id, name" and returns a PCollection of pageView structs.
func readInput(s beam.Scope, dsn string) beam.PCollection {
	s = s.Scope("readInput")

	// read from the database into a PCollection of pageView structs
	return databaseio.Read(s, "mysql", dsn, "data", reflect.TypeOf(pageView{}))
}

// a function that uses Beam to count the raw number of pageviews for each of
// the top 50 pages viewed in a given language project
func countPageViews(s beam.Scope, col beam.PCollection, params dbParams) beam.PCollection {
	s = s.Scope("countPageViews")

	// extract the page names from the input PCollection of pageviews from db
	// yields PCollection<string>
	pageviews := beam.ParDo(s, extractPage, col)

	// count the number of times each page shows up in pageviews
	// yields PCollection<string,int>
	counted := stats.Count(s, pageviews)

	// create beam constants for params". Epsilon and delta are both -1 to signify
	// that this is the normal count.
	beamParams := beam.Create(s, params)

	// using eps and del as side inputs, emit a set of wdp.TableRows
	// yields PCollection<wdp.TableRow>
	out := beam.ParDo(s, func(k string, v int, params dbParams, emit func(out wdp.TableRow)) {
		emit(wdp.TableRow{
			Name:		k,
			Views:		v,
			Lang: 		params.Lang,
			Day: 		params.Day,
			Kind: 		params.Kind,
			Epsilon: 	params.Epsilon,
			Delta:   	params.Delta,
		})
	}, counted, beam.SideInput{Input: beamParams})
	return out
}

// a function that uses Privacy on Beam to count the number of pageviews for each
// of the top 50 pages viewed in a given language project in a differentially-
// private fashion based on epsilon and delta
func privateCountPageViews(s beam.Scope, col beam.PCollection, params dbParams) beam.PCollection {
	s = s.Scope("privateCountPageViews")

	// create the privacy spec and create a PrivatePCollection
	// yields PrivatePCollection<PageView>
	spec := pbeam.NewPrivacySpec(params.Epsilon, params.Delta)
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "PV_ID")

	// extract the page names from the PrivatePCollection of pageviews
	// yields PrivatePCollection<string>
	pageviews := pbeam.ParDo(s, extractPage, pCol)

	// privately count the number of times each page shows up in pageviews
	// yields PrivatePCollection<string,int>
	counted := pbeam.Count(s, pageviews, pbeam.CountParams{ // defaults to Laplace noise
		MaxPartitionsContributed: 1, // In the scheme I've constructed, each visitor visits 1x per day (on user-level privacy, this would go to 5-10)
		MaxValue:                 1, // And they can visit a maximum of 1 page
	})

	// create constants for params
	beamParams := beam.Create(s, params)

	// using eps and del as side inputs, emit a set of wdp.TableRows
	// yields PCollection<wdp.TableRow>
	out := beam.ParDo(s, func(k string, v int64, params dbParams, emit func(out wdp.TableRow)) {
		emit(wdp.TableRow{
			Name:		k,
			Views:		int(v),
			Lang: 		params.Lang,
			Day: 		params.Day,
			Kind: 		params.Kind,
			Epsilon: 	params.Epsilon,
			Delta:   	params.Delta,
		})
	}, counted, beam.SideInput{Input: beamParams})

	return out
}

// a simple wrapper DoFn for extracting a page name from a pageView
func extractPage(p pageView) string {
	return p.Name
}

// a filter DoFn that emits a pageView if the entry's Lang matches our language
func filterLang(p pageView, lang string, emit func(pageView)) {
	if p.Lang == lang {
		emit(p)
	}
}
