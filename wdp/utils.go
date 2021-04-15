package wdp

import (
	"fmt"
    "net/http"
    "io"
    "io/ioutil"
    "encoding/json"
  	"math"
  	"strings"
    "path/filepath"
  	"strconv"
  	"time"
  	"os"
  	"encoding/csv"
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

type PageVars struct {
	Lang 		string
	MinCount 	int 
	Epsilon		float64
	Sensitivity int 
	Alpha 		float64
	PropWithin 	float64
}

var LanguageCodes = []string{"aa", "ab", "ace", "ady", "af", "ak", "als", "am", "an", "ang", "ar", "arc", "ary", "arz", "as", "ast", "atj", "av", "avk", "awa", "ay", "az", "azb", "ba", "ban", "bar", "bat-smg", "bcl", "be", "be-x-old", "bg", "bh", "bi", "bjn", "bm", "bn", "bo", "bpy", "br", "bs", "bug", "bxr", "ca", "cbk-zam", "cdo", "ce", "ceb", "ch", "cho", "chr", "chy", "ckb", "co", "cr", "crh", "cs", "csb", "cu", "cv", "cy", "da", "de", "din", "diq", "dsb", "dty", "dv", "dz", "ee", "el", "eml", "en", "eo", "es", "et", "eu", "ext", "fa", "ff", "fi", "fiu-vro", "fj", "fo", "fr", "frp", "frr", "fur", "fy", "ga", "gag", "gan", "gcr", "gd", "gl", "glk", "gn", "gom", "gor", "got", "gu", "gv", "ha", "hak", "haw", "he", "hi", "hif", "ho", "hr", "hsb", "ht", "hu", "hy", "hyw", "hz", "ia", "id", "ie", "ig", "ii", "ik", "ilo", "inh", "io", "is", "it", "iu", "ja", "jam", "jbo", "jv", "ka", "kaa", "kab", "kbd", "kbp", "kg", "ki", "kj", "kk", "kl", "km", "kn", "ko", "koi", "kr", "krc", "ks", "ksh", "ku", "kv", "kw", "ky", "la", "lad", "lb", "lbe", "lez", "lfn", "lg", "li", "lij", "lld", "lmo", "ln", "lo", "lrc", "lt", "ltg", "lv", "mai", "map-bms", "mdf", "mg", "mh", "mhr", "mi", "min", "mk", "ml", "mn", "mnw", "mr", "mrj", "ms", "mt", "mus", "mwl", "my", "myv", "mzn", "na", "nah", "nap", "nds", "nds-nl", "ne", "new", "ng", "nl", "nn", "no", "nov", "nqo", "nrm", "nso", "nv", "ny", "oc", "olo", "om", "or", "os", "pa", "pag", "pam", "pap", "pcd", "pdc", "pfl", "pi", "pih", "pl", "pms", "pnb", "pnt", "ps", "pt", "qu", "rm", "rmy", "rn", "ro", "roa-rup", "roa-tara", "ru", "rue", "rw", "sa", "sah", "sat", "sc", "scn", "sco", "sd", "se", "sg", "sh", "shn", "si", "simple", "sk", "sl", "sm", "smn", "sn", "so", "sq", "sr", "srn", "ss", "st", "stq", "su", "sv", "sw", "szl", "szy", "ta", "tcy", "te", "tet", "tg", "th", "ti", "tk", "tl", "tn", "to", "tpi", "tr", "ts", "tt", "tum", "tw", "ty", "tyv", "udm", "ug", "uk", "ur", "uz", "ve", "vec", "vep", "vi", "vls", "vo", "wa", "war", "wo", "wuu", "xal", "xh", "xmf", "yi", "yo", "za", "zea", "zh", "zh-classical", "zh-min-nan", "zh-yue", "zu"}

// get top 50 articles, then create a csv file with fake "session" data for
// beam to work with 
func InitializeSyntheticData(date, lang string) error {
	// get top 50 articles
	topFiftyArticles, err := GetGroundTruth(lang)
	if err != nil {
		return err
	}

	// create file — assume that if we're calling this function, file doesn't exist
	f, err := os.Create(fmt.Sprintf("./data/synthetic_data_%s_%s.csv", lang, date))
	defer f.Close()
	if err != nil {
		return err
	}

	// create writer and write header
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Comma = '|'
	w.Write([]string{"id", "name"})

	var totalViews = 0

	// for each article in the top 50
	for _, article := range topFiftyArticles {
		// create the requisite number of "views" of the form {id, name} and write to csv
		for j := 0; j < article.Views; j++ {
			err = w.Write([]string{strconv.Itoa(j + totalViews), article.Name})
			if err != nil {
				return err
			}
		}

		totalViews += article.Views
	}

	return nil
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
	for i, article := range apiResp.Items[0].Articles {
		if i < 50 {
			topFiftyArticles[i] = article
		} else {
			break
		}
	}

	return topFiftyArticles, nil
}

// Provide a qualitative explanation for what a particular epsilon value means.
func qualEps(eps, p float64) float64 {
	 //    Recommended description:
	 //        If someone believed a user was in the data with probability p,
	 //        then at most after seeing the data they will be qual_eps(eps, p) certain
	 //        (assuming the sensitivity value is correct).
	 //        e.g., for eps=1; p=0.5, they'd go from 50% certainty to at most 73.1% certainty.

	 //    Parameters:
	 //        eps: epsilon value that quantifies "privacy" level of differential privacy.
	 //        p: initial belief that a given user is in the data -- e.g.,:
	 //            0.5 represents complete uncertainty (50/50 chance)
	 //            0.01 represents high certainty the person isn't in the data
	 //            0.99 represents high certainty the person is in the data
	if p > 0 && p < 1 {
        return (math.Exp(eps) * p) / (1 + ((math.Exp(eps) - 1) * p))
	} else {
        return -1
    }
}

func aggregationThreshold(sensitivity, eps int, alpha, propWithin float64) float64 {
	// Same as doAggregate but determines threshold where data is deemed 'too noisy'.
	var rank = alpha / 2
	var lbda = sensitivity / eps
	// get confidence interval; this is symmetric where `lower bound = noisedX - ci` and `upper bound = noisedX + ci`
	var ci = math.Abs(float64(lbda) * math.Log(2*rank))
	return math.Ceil(ci / propWithin)
}

func doAggregate(noisedX, sensitivity, eps int, alpha, prop_within float64) string {
    // Check whether noisy data X is at least (100 * alpha)% of being within Y% of true value.
    // Doesn't use true value (only noisy value and parameters) so no privacy cost to this.
    // Should identify in advance what threshold -- e.g., 50% probability within 25% of actual value -- in advance
    // to determine whether to keep the data or suppress it until it can be further aggregated so more signal to noise.
    // See a more complete description in the paper below for how to effectively use this data.

    // Based on:
    // * Description: https://arxiv.org/pdf/2009.01265.pdf#section.4
    // * Code: https://github.com/google/differential-privacy/blob/main/java/main/com/google/privacy/differentialprivacy/LaplaceNoise.java#L127

    // Parameters:
    //     noisedX: the count after adding Laplace noise
    //     sensitivity: L1 sensitivity for Laplace
    //     eps: selected epsilon value
    //     alpha: how confident (0.5 = 50%; 0.1 = 90%) should we be that the noisy data is within (100 * prop_within)% of the true data?
    //     prop_within: how close (0.25 = 25%) to actual value should we expect the noisy data to be?
    
    // Divide alpha by 2 because two-tailed
    var rank = alpha / 2
    var lbda = sensitivity / eps
    // get confidence interval; this is symmetric where `lower bound = noisedX - ci` and `upper bound = noisedX + ci`
    var ci = math.Abs(float64(lbda) * math.Log(2*rank))
    if ci > (prop_within * float64(noisedX)) {
        return "Yes"
    } else {
        return "No"
    }
}

// validation of query params and inputs
func validateLang(lang string) bool {
	for _, l := range LanguageCodes {
		if lang == l {
			return true
		}
	}
	return false
}

// validation of epsilon value
func validateEpsilon(epsilon float64) bool {
	if !math.IsInf(epsilon, 1) && !math.IsNaN(epsilon) && epsilon > 0 {
		return true
	}
	return false
}

// validation of sensitivity value
func validateSensitivity(sensitivity int) bool {
	if !math.IsInf(float64(sensitivity), 1) && !math.IsNaN(float64(sensitivity)) && sensitivity > 0 {
		return true
	}
	return false
}

// validation of mincount value
func validateMinCount(mincount int) bool {
	if mincount >= 0 {
		return true
	}
	return false
}

// validation of alpha value
func validateAlpha(alpha float64) bool {
	if alpha > 0 && alpha < 1 {
		return true
	}
	return false
}

// validation of prop within value
func validatePropWithin(propWithin float64) bool {
	if propWithin > 0 && propWithin < 1 {
		return true
	}
	return false	
}

// compose all previous validation functions to validate all inputs
func ValidateApiArgs(r *http.Request) (PageVars, error) {
	request := r.URL.Query()

	var pvs = PageVars{Lang:			"en",
					   MinCount: 		int(0),
					   Epsilon: 		float64(1),
					   Sensitivity: 	int(1),
					   Alpha: 			float64(0.5),
					   PropWithin: 		float64(0.25)}

	if _, ok := request["lang"]; ok {
		if validateLang(strings.ToLower(request["lang"][0])) {
			pvs.Lang = strings.ToLower(request["lang"][0])
		}
	}

	// not currently used
	if _, ok := request["mincount"]; ok {
		i, err := strconv.Atoi(request["mincount"][0])
		if err != nil {
			return pvs, err
		}
		if validateMinCount(i) {
			pvs.MinCount = i
		}
	}

	if _, ok := request["eps"]; ok {
		f, err := strconv.ParseFloat(request["eps"][0], 64)
		if err != nil {
			return pvs, err
		}
		if validateEpsilon(f) {
			pvs.Epsilon = f
		}
	}

	if _, ok := request["sensitivity"]; ok {
		i, err := strconv.Atoi(request["sensitivity"][0])
		if err != nil {
			return pvs, err
		}
		if validateSensitivity(i) {
			pvs.Sensitivity = i
		}
	}

	if _, ok := request["alpha"]; ok {
		f, err := strconv.ParseFloat(request["alpha"][0], 64)
		if err != nil {
			return pvs, err
		}
		if validateAlpha(f) {
			pvs.Alpha = f
		}
	}

	if _, ok := request["propWithin"]; ok {
		f, err := strconv.ParseFloat(request["propWithin"][0], 64)
		if err != nil {
			return pvs, err
		}
		if validatePropWithin(f) {
			pvs.PropWithin = f
		}
	}

	return pvs, nil
}

// enable CORS headers for the API
func EnableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}

// helper function to remove all files/subdirs in a directory
func RemoveOldContents(date, dir string) error {
	path, err := os.Getwd()
	if err != nil {
		return err
	}

	path = filepath.Join(path, dir)

    files, err := ioutil.ReadDir(dir)
    for _, f := range files {
    	if !strings.HasSuffix(f.Name(), fmt.Sprintf("%s.csv", date)) {
    		err = os.Remove(filepath.Join(path, f.Name()))
    		if err != nil {
    			return err
    		}
    	}
    }

    return nil
}
