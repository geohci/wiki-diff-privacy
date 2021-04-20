// Functions for the validation of inputs from an end user

package wdp

import (
	"log"
	"math"
	"net/http"
	"strings"
	"strconv"
)

// all wiki size info from: https://meta.wikimedia.org/wiki/List_of_Wikipedias

// all languages
// var LanguageCodes = []string{"aa", "ab", "ace", "ady", "af", "ak", "als", "am", "an", "ang", "ar", "arc", "ary", "arz", "as", "ast", "atj", "av", "avk", "awa", "ay", "az", "azb", "ba", "ban", "bar", "bat-smg", "bcl", "be", "be-x-old", "bg", "bh", "bi", "bjn", "bm", "bn", "bo", "bpy", "br", "bs", "bug", "bxr", "ca", "cbk-zam", "cdo", "ce", "ceb", "ch", "cho", "chr", "chy", "ckb", "co", "cr", "crh", "cs", "csb", "cu", "cv", "cy", "da", "de", "din", "diq", "dsb", "dty", "dv", "dz", "ee", "el", "eml", "en", "eo", "es", "et", "eu", "ext", "fa", "ff", "fi", "fiu-vro", "fj", "fo", "fr", "frp", "frr", "fur", "fy", "ga", "gag", "gan", "gcr", "gd", "gl", "glk", "gn", "gom", "gor", "got", "gu", "gv", "ha", "hak", "haw", "he", "hi", "hif", "ho", "hr", "hsb", "ht", "hu", "hy", "hyw", "hz", "ia", "id", "ie", "ig", "ii", "ik", "ilo", "inh", "io", "is", "it", "iu", "ja", "jam", "jbo", "jv", "ka", "kaa", "kab", "kbd", "kbp", "kg", "ki", "kj", "kk", "kl", "km", "kn", "ko", "koi", "kr", "krc", "ks", "ksh", "ku", "kv", "kw", "ky", "la", "lad", "lb", "lbe", "lez", "lfn", "lg", "li", "lij", "lld", "lmo", "ln", "lo", "lrc", "lt", "ltg", "lv", "mai", "map-bms", "mdf", "mg", "mh", "mhr", "mi", "min", "mk", "ml", "mn", "mnw", "mr", "mrj", "ms", "mt", "mus", "mwl", "my", "myv", "mzn", "na", "nah", "nap", "nds", "nds-nl", "ne", "new", "ng", "nl", "nn", "no", "nov", "nqo", "nrm", "nso", "nv", "ny", "oc", "olo", "om", "or", "os", "pa", "pag", "pam", "pap", "pcd", "pdc", "pfl", "pi", "pih", "pl", "pms", "pnb", "pnt", "ps", "pt", "qu", "rm", "rmy", "rn", "ro", "roa-rup", "roa-tara", "ru", "rue", "rw", "sa", "sah", "sat", "sc", "scn", "sco", "sd", "se", "sg", "sh", "shn", "si", "simple", "sk", "sl", "sm", "smn", "sn", "so", "sq", "sr", "srn", "ss", "st", "stq", "su", "sv", "sw", "szl", "szy", "ta", "tcy", "te", "tet", "tg", "th", "ti", "tk", "tl", "tn", "to", "tpi", "tr", "ts", "tt", "tum", "tw", "ty", "tyv", "udm", "ug", "uk", "ur", "uz", "ve", "vec", "vep", "vi", "vls", "vo", "wa", "war", "wo", "wuu", "xal", "xh", "xmf", "yi", "yo", "za", "zea", "zh", "zh-classical", "zh-min-nan", "zh-yue", "zu"}

// a list of all languages with 2000-6000 active wikipedia users (for ease of computation)
// var LanguageCodes = []string{"ar", "pl", "nl", "uk", "sv", "vi", "fa", "tr", "he", "cs", "id", "ko", "fi"}

// a list of the largest wikipedias
// var LanguageCodes = []string{"en", "es", "fr", "de", "zh", "ru", "pt", "it", "ar", "ja", "nl", "pl", "tr", "id", "fa"}

// test list
var LanguageCodes = []string{"fa"}


type PageVars struct {
	Lang 		string
	MinCount 	int 
	Epsilon		float64
	Delta		float64
	Sensitivity int
	Alpha 		float64
	PropWithin 	float64
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
	// if !math.IsInf(epsilon, 1) && !math.IsNaN(epsilon) && epsilon > 0 {

	// make sure that epsilon is a number and it isn't inf
	if !math.IsInf(epsilon, 1) && !math.IsNaN(epsilon) {
		// if it equals a valid epsilon, return true
		for _, e := range Epsilon {
			if e == epsilon {
				return true
			}
		}
	}
	return false
}

// validation of delta value
func validateDelta(delta float64) bool {
	// if !math.IsInf(epsilon, 1) && !math.IsNaN(epsilon) && epsilon > 0 {
		
	// make sure that delta is a number and it isn't inf
	if !math.IsInf(delta, 1) && !math.IsNaN(delta) {
		// if it equals a valid delta, return true
		for _, d := range Delta {
			if d == delta {
				log.Print("delta validated")
				return true
			}
		}
	}
	log.Print("delta not validated")
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
					   Delta: 			float64(0.000001),
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

	if _, ok := request["delta"]; ok {
		f, err := strconv.ParseFloat(request["delta"][0], 64)
		if err != nil {
			return pvs, err
		}
		if validateDelta(f) {
			pvs.Delta = f
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