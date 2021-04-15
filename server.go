package main

import (
    "fmt"
    "net/http"
    "io"
    "io/ioutil"
    "encoding/json"
    "html/template"
  	"log"
  	"math"
  	"strings"
    "path/filepath"
  	"strconv"
  	"time"
  	"os"
  	"encoding/csv"
  	"reflect"
  	"context"

	"flag"
	log "github.com/golang/glog"
	"github.com/google/differential-privacy/privacy-on-beam/codelab"
	"github.com/apache/beam/sdks/go/pkg/beam"

	// The following import is required for accessing local files.
	_ "github.com/apache/beam/sdks/go/pkg/beam/io/filesystem/local"

	"github.com/apache/beam/sdks/go/pkg/beam/runners/direct"
)

func init() {
	beam.RegisterType(reflect.TypeOf((*normalizeOutputCombineFn)(nil)))
	beam.RegisterType(reflect.TypeOf(outputAccumulator{}))
	beam.RegisterFunction(convertToPairFn)
}

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

type PageVars struct {
	Lang 		string
	MinCount 	int 
	Epsilon		float64
	Sensitivity int 
	Alpha 		float64
	PropWithin 	float64
}

type DPArticle struct {
	Name	string
	Views 	int
	Rank 	int
	DPView	int
	DPRank	int
	DoAgg 	string
}

var LanguageCodes = []string{"aa", "ab", "ace", "ady", "af", "ak", "als", "am", "an", "ang", "ar", "arc", "ary", "arz", "as", "ast", "atj", "av", "avk", "awa", "ay", "az", "azb", "ba", "ban", "bar", "bat-smg", "bcl", "be", "be-x-old", "bg", "bh", "bi", "bjn", "bm", "bn", "bo", "bpy", "br", "bs", "bug", "bxr", "ca", "cbk-zam", "cdo", "ce", "ceb", "ch", "cho", "chr", "chy", "ckb", "co", "cr", "crh", "cs", "csb", "cu", "cv", "cy", "da", "de", "din", "diq", "dsb", "dty", "dv", "dz", "ee", "el", "eml", "en", "eo", "es", "et", "eu", "ext", "fa", "ff", "fi", "fiu-vro", "fj", "fo", "fr", "frp", "frr", "fur", "fy", "ga", "gag", "gan", "gcr", "gd", "gl", "glk", "gn", "gom", "gor", "got", "gu", "gv", "ha", "hak", "haw", "he", "hi", "hif", "ho", "hr", "hsb", "ht", "hu", "hy", "hyw", "hz", "ia", "id", "ie", "ig", "ii", "ik", "ilo", "inh", "io", "is", "it", "iu", "ja", "jam", "jbo", "jv", "ka", "kaa", "kab", "kbd", "kbp", "kg", "ki", "kj", "kk", "kl", "km", "kn", "ko", "koi", "kr", "krc", "ks", "ksh", "ku", "kv", "kw", "ky", "la", "lad", "lb", "lbe", "lez", "lfn", "lg", "li", "lij", "lld", "lmo", "ln", "lo", "lrc", "lt", "ltg", "lv", "mai", "map-bms", "mdf", "mg", "mh", "mhr", "mi", "min", "mk", "ml", "mn", "mnw", "mr", "mrj", "ms", "mt", "mus", "mwl", "my", "myv", "mzn", "na", "nah", "nap", "nds", "nds-nl", "ne", "new", "ng", "nl", "nn", "no", "nov", "nqo", "nrm", "nso", "nv", "ny", "oc", "olo", "om", "or", "os", "pa", "pag", "pam", "pap", "pcd", "pdc", "pfl", "pi", "pih", "pl", "pms", "pnb", "pnt", "ps", "pt", "qu", "rm", "rmy", "rn", "ro", "roa-rup", "roa-tara", "ru", "rue", "rw", "sa", "sah", "sat", "sc", "scn", "sco", "sd", "se", "sg", "sh", "shn", "si", "simple", "sk", "sl", "sm", "smn", "sn", "so", "sq", "sr", "srn", "ss", "st", "stq", "su", "sv", "sw", "szl", "szy", "ta", "tcy", "te", "tet", "tg", "th", "ti", "tk", "tl", "tn", "to", "tpi", "tr", "ts", "tt", "tum", "tw", "ty", "tyv", "udm", "ug", "uk", "ur", "uz", "ve", "vec", "vep", "vi", "vls", "vo", "wa", "war", "wo", "wuu", "xal", "xh", "xmf", "yi", "yo", "za", "zea", "zh", "zh-classical", "zh-min-nan", "zh-yue", "zu"}


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

	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
	err = wdp.RemoveOldContents(yesterday, "data/")
	if err != nil {
		log.Print("error removing contents of data folder: ", err)
	}
	f, err = os.Open(fmt.Sprintf("./data/synthetic_data_%s_%s.csv", vars.Lang, yesterday))
	defer f.Close()
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

		pvs := wdp.ReadInput(s, f)
		rawOutput := wdp.CountPageViews(s, pvs)
		// dpOutput := wdp.PrivateCountPageViews(s, pvs)

		wdp.WriteOutput(s, rawOutput, fmt.Sprintf("./data/output_%s_%s.csv", vars.Lang, yesterday)
		// wdp.WriteOutput(s, dpOutput, fmt.Sprintf("./data/output_%s_%s.csv", vars.Lang, yesterday)
		// Execute pipeline.
		err := direct.Execute(context.Background(), p)
		if err != nil {
			log.Exitf("Execution of pipeline failed: %v", err)
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


// functions and types for aggregation in beam
// from codelab example, with light changes
type pair struct {
	K string
	V float64
}

func convertToPairFn(k string, v beam.V) (pair, error) {
	switch v := v.(type) {
	case int:
		return pair{K: k, V: float64(v)}, nil
	case int64:
		return pair{K: k, V: float64(v)}, nil
	case float64:
		return pair{K: k, V: v}, nil
	default:
		return pair{}, fmt.Errorf("expected int, int64 or float64 for value type, got %v", v)
	}
}

type outputAccumulator struct {
	PageToValue map[string]float64
}

type normalizeOutputCombineFn struct{}

func (fn *normalizeOutputCombineFn) CreateAccumulator() outputAccumulator {
	pageToValue := make(map[string]float64)
	topFiftyArticles, _ := GetGroundTruth(lang)
	for _, article := range topFiftyArticles {
		pageToValue[article] = 0
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

func (fn *normalizeOutputCombineFn) ExtractOutput(a outputAccumulator) string {
	var lines []string
	for k, v := range a.PageToValue {
		lines = append(lines, fmt.Sprintf("%d %f", k, v))
	}
	return strings.Join(lines, "\n")
}
