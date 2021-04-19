package main

import (
	"fmt"
	"regexp"
	"reflect"
	"math"
	"time"
	"strings"
	"strconv"
	"log"
	"context"

	"github.com/htried/wiki-diff-privacy/wdp"

	"github.com/apache/beam/sdks/go/pkg/beam"

	// The following import is required for accessing local files.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	// "github.com/apache/beam/sdks/go/pkg/beam/io/textio"
	"github.com/apache/beam/sdks/go/pkg/beam/io/databaseio"
)

// initialize functions and types to be referenced in beam
func init() {
	beam.RegisterType(reflect.TypeOf((*pageView)(nil)))
	// beam.RegisterFunction(createPageViewsFn)
	beam.RegisterFunction(extractPage)
}

var epsilon = []float64{0.1, 0.5, 1, 5}
var delta = []float64{math.Pow10(-4), math.Pow10(-2), math.Pow10(-1), 1}

// struct to represent 1 synthetic pageview
type pageView struct {
	ID 		string
	Name 	string
}

type output struct {
	Name 	string
	Views 	int
	Epsilon float64
	Delta 	float64
}

func main() {
	err := processLanguage("ak")
	if err != nil {
		log.Printf("Error processing language %s\n", err)
		return
	}
}


func processLanguage(lang string) error {
	dpMap := make(map[string]beam.PCollection)
	dsn, err := wdp.DSN("wdp")
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return err
    }

    err = tableSetUp(lang)
    if err != nil {
    	log.Printf("Error %s when setting up tables\n", err)
    }

	beam.Init()
	p := beam.NewPipeline()
	s := p.Root()

	pvs := readInput(s, dsn, lang)
	normalCount := countPageViews(s, pvs)

	for _, eps := range epsilon {
		for _, del := range delta {
			key := "e" + strconv.FormatFloat(eps, 'f', -1, 64) + "_d" + strconv.FormatFloat(del, 'f', -1, 64)
			dpMap[key] = privateCountPageViews(s, pvs, eps, del)
		}
	}

	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")

	// write to normal db
    tbl_name := fmt.Sprintf("output_e0_d0_%s_%s", lang, yesterday)
	databaseio.Write(s, "mysql", dsn, tbl_name, []string{}, normalCount)

	for params, privCount := range dpMap {
		tbl_name = fmt.Sprintf("output_%s_%s_%s", params, lang, yesterday)
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

func tableSetUp(lang string) error {
	db, err := wdp.DBConnection()
	if err != nil {
    	log.Printf("Error %s when connecting to db\n", err)
    	return err
    }
    defer db.Close()

    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")

    // create table for normal counts
    tbl_name := fmt.Sprintf("output_e0_d0_%s_%s", lang, yesterday)
    err = wdp.CreateTable(db, tbl_name)
    if err != nil {
    	log.Printf("Error %s when creating new table\n", err)
    	return err
    }

    // create tables for various combos of epsilon and delta
    for _, eps := range epsilon {
    	for _, del := range delta {
    		var e = strconv.FormatFloat(eps, 'f', -1, 64)
    		var d = strconv.FormatFloat(del, 'f', -1, 64)
		    tbl_name := fmt.Sprintf("output_e%s_d%s_%s_%s", e, d, lang, yesterday)

		    err = wdp.CreateTable(db, tbl_name)
		    if err != nil {
		    	log.Printf("Error %s when creating new table\n", err)
		    	return err
		    }
    	}
    }

    return nil
}

// functions for aggregation in beam
// mostly from codelab example, with light changes

// readInput reads from a database detailing page views in the form
// of "id, name" and returns a PCollection of pageView structs.
func readInput(s beam.Scope, dsn,  lang string) beam.PCollection {
	s = s.Scope("readInput")
    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
    lang = strings.ReplaceAll(lang, "-", "_")
    tbl_name := fmt.Sprintf("data_%s_%s", lang, yesterday)
	return databaseio.Read(s, "mysql", dsn, tbl_name, reflect.TypeOf((*pageView)(nil))) // maybe just pageView
	// return beam.ParDo(s, createPageViewsFn, lines)
}


// func createPageViewsFn(line string, emit func(pageView)) error {
// 	// Skip the column headers line
// 	notHeader, err := regexp.MatchString("[0-9]", line)
// 	if err != nil {
// 		return err
// 	}
// 	if !notHeader {
// 		return nil
// 	}

// 	cols := strings.Split(line, "|")
// 	if len(cols) != 2 {
// 		return fmt.Errorf("got %d number of columns in line %q, expected 2", len(cols), line)
// 	}
// 	id := cols[0]
// 	name := cols[1]
// 	emit(pageView{
// 		ID:		id,
// 		Name: 	name,
// 	})
// 	return nil
// }

func countPageViews(s beam.Scope, col beam.PCollection) beam.PCollection {
	s = s.Scope("countPageViews")
	pageviews := beam.ParDo(s, extractPage, col)
	counted := stats.Count(s, pageviews)
	return counted

	// formatted := beam.ParDo(s, func(page string, count int) string {
	// 	return fmt.Sprintf("%s,%v", page, count)
	// }, counted)

	// return formatted
}

func privateCountPageViews(s beam.Scope, col beam.PCollection, epsilon, delta float64) beam.PCollection {
	s = s.Scope("privateCountPageViews")

	spec := pbeam.NewPrivacySpec(epsilon, delta)
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "ID")

	pageviews := pbeam.ParDo(s, extractPage, pCol)
	counted := pbeam.Count(s, pageviews, pbeam.CountParams{
		MaxPartitionsContributed:	1, // In the scheme I've constructed, each visitor visits 1x per day
		MaxValue: 					1, // And they can visit a maximum of 1 page
	})
	return counted

	// formatted := beam.ParDo(s, func(page string, count int64) string {
	// 	return fmt.Sprintf("%s,%v", page, count)
	// }, counted)

	// return formatted
}

func extractPage(p pageView) string {
	reg, err := regexp.Compile(`[^a-zA-Z0-9\_:]+`)
    if err != nil {
        log.Print("error with creating regex: ", err)
    }
    
	return reg.ReplaceAllString(p.Name, "")
}

func 
