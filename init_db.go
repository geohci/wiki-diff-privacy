// script for initializing synthetic databases and output databases. should be the
// first thing that is run in this package to get everything started. we leave the
// output table completely blank besides initializing it.

package main

import (
	// "fmt"
	"log"
	// "strings"
	"time"

	"github.com/htried/wiki-diff-privacy/wdp"
)

// func main() {
// 	start := time.Now()
// 	// get a connection to the db
// 	db, err := wdp.DBConnection()
// 	if err != nil {
// 		log.Printf("Error %s when getting db connection", err)
// 		return
// 	}
// 	defer db.Close()
// 	log.Printf("Successfully connected to database")

// 	// get the date of the tables you want to make
// 	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")

// 	// for each language
// 	for _, lang := range wdp.LanguageCodes {
// 		langStart := time.Now()

// 		// replace hypens with underscores (for syntactical reasons)
// 		lang = strings.ReplaceAll(lang, "-", "_")

// 		// create the synthetic data table
// 		data_tbl_name := fmt.Sprintf("data_%s_%s", lang, yesterday)
// 		err = wdp.CreateTable(db, data_tbl_name)
// 		if err != nil {
// 			log.Printf("Create table failed with error %s", err)
// 			return
// 		}

// 		// create the output table
// 		out_tbl_name := fmt.Sprintf("output_%s_%s", lang, yesterday)
// 		err = wdp.CreateTable(db, out_tbl_name)
// 		if err != nil {
// 			log.Printf("Create table failed with error %s", err)
// 			return
// 		}

// 		// get the top fifty articles for this language
// 		topFiftyArticles, err := wdp.GetGroundTruth(lang)
// 		if err != nil {
// 			log.Printf("getGroundTruth failed with error %s", err)
// 			return
// 		}

// 		// batch insert faux data into the synthetic data table
// 		err = wdp.BatchInsert(db, data_tbl_name, topFiftyArticles)

// 		log.Printf("Time to init %s dbs: %v seconds\n", lang, time.Now().Sub(langStart).Seconds())

// 	}
// 	log.Printf("Time to init all dbs: %v seconds\n", time.Now().Sub(start).Seconds())
// }

func main() {
	start := time.Now()
	// get a connection to the db
	db, err := wdp.DBConnection()
	if err != nil {
		log.Printf("Error %s when getting db connection", err)
		return
	}
	defer db.Close()
	log.Printf("Successfully connected to database")

	// create the synthetic data table
	err = wdp.CreateTable(db, "data")
	if err != nil {
		log.Printf("Create table failed with error %s", err)
		return
	}

	// create the output table
	err = wdp.CreateTable(db, "output")
	if err != nil {
		log.Printf("Create table failed with error %s", err)
		return
	}

	// get the date of the tables you want to make
	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// for each language
	for _, lang := range wdp.LanguageCodes {
		langStart := time.Now()

		// get the top fifty articles for this language
		topFiftyArticles, err := wdp.GetGroundTruth(lang)
		if err != nil {
			log.Printf("getGroundTruth failed with error %s", err)
			return
		}

		// batch insert faux data into the synthetic data table
		err = wdp.BatchInsert(db, "data", yesterday, lang, topFiftyArticles)

		log.Printf("Time to init %s rows: %v seconds\n", lang, time.Now().Sub(langStart).Seconds())

	}
	log.Printf("Time to init all languages: %v seconds\n", time.Now().Sub(start).Seconds())
}

