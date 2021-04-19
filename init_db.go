package main

import (
	"log"
	"time"
	"fmt"
	"github.com/htried/wiki-diff-privacy/wdp"
    "strings"
)

func main() {  
    db, err := wdp.DBConnection()
    if err != nil {
        log.Printf("Error %s when getting db connection", err)
        return
    }
    defer db.Close()
    log.Printf("Successfully connected to database")

    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
    for _, lang := range wdp.LanguageCodes {
        lang = strings.ReplaceAll(lang, "-", "_")

    	tbl_name := fmt.Sprintf("data_%s_%s", lang, yesterday)
    	err = wdp.CreateSyntheticDataTable(db, tbl_name)
	    if err != nil {
	        log.Printf("Create table failed with error %s", err)
	        return
	    }

	    topFiftyArticles, err := wdp.GetGroundTruth(lang)
		if err != nil {
			log.Printf("getGroundTruth failed with error %s", err)
			return 
		}

	    err = wdp.BatchInsert(db, tbl_name, topFiftyArticles)
    }
}
