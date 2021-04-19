package main

import (
	"log"
	"github.com/htried/wiki-diff-privacy/wdp"
)

func main() {  
    db, err := wdp.DBConnection()
    if err != nil {
        log.Printf("Error %s when getting db connection", err)
        return
    }
    defer db.Close()
    log.Printf("Successfully connected to database")

    err = wdp.DropSyntheticData(db)
    if err != nil {
    	log.Printf("Error %s when dropping synthetic data", err)
    	return
    }

    err = wdp.DropOldData(db)
    if err != nil {
    	log.Printf("Error %s when dropping data from previous days", err)
    	return
    }
}