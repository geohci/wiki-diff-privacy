// script for cleaning everything up once we've run the whole data pipeline, so
// as to ensure that we don't have a huge amount of synthetic data or previous days
// of data building up and crashing the server.

package main

import (
	"github.com/htried/wiki-diff-privacy/wdp"
	"log"
	"time"
)

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

	var yesterday = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	log.Printf(yesterday)

	// drop old synthetic data from before yesterday
	err = wdp.DropOldData(db, "data", yesterday)
	if err != nil {
		log.Printf("Error %s when dropping synthetic data", err)
		return
	}

	// drop old output data from previous days
	err = wdp.DropOldData(db, "output", yesterday)
	if err != nil {
		log.Printf("Error %s when dropping synthetic data", err)
		return
	}

	log.Printf("Time to clean up all databases: %v seconds\n", time.Now().Sub(start).Seconds())
}

