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

	"github.com/apache/beam/sdks/go/pkg/beam"

	// The following import is required for accessing local files.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
	"github.com/htried/wiki-diff-privacy/wdp"
	
	"github.com/google/differential-privacy/privacy-on-beam/pbeam"
	"github.com/apache/beam/sdks/go/pkg/beam/transforms/stats"
	"github.com/apache/beam/sdks/go/pkg/beam/io/textio"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*PageView)(nil)))
	beam.RegisterType(reflect.TypeOf((*normalizeOutputCombineFn)(nil)))
	beam.RegisterType(reflect.TypeOf(outputAccumulator{}))
	beam.RegisterFunction(createPageViewsFn)
	beam.RegisterFunction(convertToPairFn)
	beam.RegisterFunction(extractPage)
}


type DPArticle struct {
	Name	string
	Views 	int
	Rank 	int
	DPView	int
	DPRank	int
	DoAgg 	string
}

type PageView struct {
	ID 		string
	Name 	string
}

var TopFiftyArticles [50]wdp.Article


func Index(w http.ResponseWriter, r *http.Request) {
	vars, err := wdp.ValidateApiArgs(r)
	if err != nil {
		log.Print("error validating API arguments: ", err)
	}

	t, err := template.ParseFiles("templates/index_go.html")
	if err != nil {
		log.Print("error parsing template index_go.html: ", err)
	}

	err = t.Execute(w, vars)
	if err != nil {
		log.Print("error executing template index_go.html: ", err)
	}
}


func PageViews(w http.ResponseWriter, r *http.Request) {
	wdp.EnableCors(&w)

	vars, err := wdp.ValidateApiArgs(r)
	if err != nil {
		log.Print("error validating API arguments: ", err)
	}

	TopFiftyArticles, err = wdp.GetGroundTruth(vars.Lang)
	if err != nil {
		log.Print("error getting ground truth from API: ", err)
	}

	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
	err = wdp.RemoveOldContents(yesterday, "data/")
	if err != nil {
		log.Print("error removing contents of data folder: ", err)
	}

	fname := fmt.Sprintf("./data/synthetic_data_%s_%s.csv", vars.Lang, yesterday)
	outname := fmt.Sprintf("./data/output_%s_%s.csv", vars.Lang, yesterday)
	_, err = os.Stat(fname)
	if os.IsNotExist(err) {
		err = wdp.InitializeSyntheticData(yesterday, vars.Lang)
		if err != nil {
			log.Print("error initializing synthetic data from yesterday: ", err)
		}
	} else if err != nil {
		log.Print("error stat-ing file: ", err)
	} else {
		// open file and run beam pipeline here
		beam.Init()
		p := beam.NewPipeline()
		s := p.Root()

		pvs := readInput(s, fname)
		log.Print("read")
		pageviews := beam.ParDo(s, extractPage, pvs)
		log.Print("extracted")
		counted := stats.Count(s, pageviews)
		log.Print("counted")
		formatted := beam.ParDo(s, func(page string, count int) string {
			return fmt.Sprintf("%s: %v", page, count)
		}, counted)
		log.Print("formatted")
		textio.Write(s, outname, formatted)
		log.Print("written")

		// rawOutput := countPageViews(s, pvs)
		// dpOutput := privateCountPageViews(s, pvs)

		// writeOutput(s, rawOutput, outname)
		// wdp.WriteOutput(s, dpOutput, outname)
		// Execute pipeline.
		_, err = direct.Execute(context.Background(), p)
		if err != nil {
			log.Print("Execution of pipeline failed: %v", err)
		}
	}
}


// bind functions to paths and start listener
func main() {
	// undo at the end
    // http.HandleFunc("/", Index)
    // http.HandleFunc("/api/v1/pageviews", PageViews)
    http.HandleFunc("/", PageViews)
    log.Fatal(http.ListenAndServe(":5000", nil))
}




func createPageViewsFn(line string, emit func(PageView)) error {
	// Skip the column headers line
	notHeader, err := regexp.MatchString("[0-9]", line)
	if err != nil {
		return err
	}
	if !notHeader {
		return nil
	}

	cols := strings.Split(line, "|")
	if len(cols) != 2 {
		return fmt.Errorf("got %d number of columns in line %q, expected 2", len(cols), line)
	}
	id := cols[0]
	name := cols[1]
	emit(PageView{
		ID:		id,
		Name: 	name,
	})
	return nil
}

// readInput reads from a .csv file detailing page views in the form
// of "id, name" and returns a PCollection of Visit structs.

// from the privacy on beam codelab tutorial
func readInput(s beam.Scope, input string) beam.PCollection {
	s = s.Scope("readInput")
	lines := textio.Read(s, input)
	return beam.ParDo(s, createPageViewsFn, lines)
}

func writeOutput(s beam.Scope, output beam.PCollection, outputTextName string) {
	s = s.Scope("writeOutput")
	output = beam.ParDo(s, convertToPairFn, output)
	formattedOutput := beam.Combine(s, &normalizeOutputCombineFn{}, output)
	log.Print("combined: ", formattedOutput)
	textio.Write(s, outputTextName, formattedOutput)
}


// functions and types for aggregation in beam
// from codelab example, with light changes
type pair struct {
	K string
	V int
}

func convertToPairFn(k string, v int) (pair, error) {

// func convertToPairFn(k string, v beam.V) (pair, error) {
	// switch v := v.(type) {
	// case int:
	// 	return pair{K: k, V: float64(v)}, nil
	// case int64:
	// 	return pair{K: k, V: float64(v)}, nil
	// case float64:
	return pair{K: k, V: v}, nil
	// default:
	// 	return pair{}, fmt.Errorf("expected int, int64 or float64 for value type, got %v", v)
	// }
}

type outputAccumulator struct {
	PageToValue map[string]int
}

type normalizeOutputCombineFn struct{}

func (fn *normalizeOutputCombineFn) CreateAccumulator() outputAccumulator {
	pageToValue := make(map[string]int)
	for _, article := range TopFiftyArticles {
		pageToValue[article.Name] = 0
	}
	return outputAccumulator{pageToValue}
}

func (fn *normalizeOutputCombineFn) AddInput(a outputAccumulator, p pair) outputAccumulator {
	a.PageToValue[p.K] = p.V
	return a
}

func (fn *normalizeOutputCombineFn) MergeAccumulators(a, b outputAccumulator) outputAccumulator {
	for k, v := range b.PageToValue {
		if v != 0 {
			a.PageToValue[k] = v
		}
	}
	return a
}

func (fn *normalizeOutputCombineFn) extractOutput(a outputAccumulator) string {
	var lines []string
	for k, v := range a.PageToValue {
		lines = append(lines, fmt.Sprintf("%s, %v", k, v))
	}
	return strings.Join(lines, "\n")
}


func countPageViews(s beam.Scope, col beam.PCollection) beam.PCollection {
	s = s.Scope("countPageViews")
	pageviews := beam.ParDo(s, extractPage, col)
	viewsPerPage := stats.Count(s, pageviews)
	return viewsPerPage
}

func privateCountPageViews(s beam.Scope, col beam.PCollection, epsilon float64, sensitivity int) beam.PCollection {
	s = s.Scope("countPageViews")

	spec := pbeam.NewPrivacySpec(epsilon, float64(sensitivity))
	pCol := pbeam.MakePrivateFromStruct(s, col, spec, "id")

	pageviews := pbeam.ParDo(s, extractPage, pCol)
	viewsPerPage := pbeam.Count(s, pageviews, pbeam.CountParams{
		MaxPartitionsContributed:	1, // In the scheme I've constructed, each visitor visits once per day
		MaxValue: 					1, // And they can visit a maximum of one page
	})
	return viewsPerPage
}

func extractPage(p PageView) string {
	reg, err := regexp.Compile(`[^a-zA-Z0-9\_]+`)
    if err != nil {
        log.Print("error with creating regex: ", err)
    }
    
	return reg.ReplaceAllString(p.Name, "")
}
