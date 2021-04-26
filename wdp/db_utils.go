// implements the backend, often-used pieces of the database functionality for
// the web app

package wdp

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"context"
	"log"
	"time"
	"fmt"
	"strings"
    "os"
    "bufio"
)

// gets the DSN based on an input string
func DSN(dbName string) (string, error) {
    // NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
    // f, err := os.Open("/Users/haltriedman/replica.my.cnf") // LOCAL
    // f, err := os.Open("/data/project/diff-privacy-beam/replica.my.cnf") // TOOLFORGE
    f, err := os.Open("/home/htriedman/replica.my.cnf") // CLOUD VPS
    defer f.Close()
    if err != nil {
        fmt.Printf("Error %s when opening replica file", err)
        return "", err
    }

    scanner := bufio.NewScanner(f)

    scanner.Split(bufio.ScanLines)
    var username string
    var password string
  
    for scanner.Scan() {
        str_split := strings.Split(scanner.Text(), " = ")
        if str_split[0] == "user" {
            username = str_split[1]
        } else if str_split[0] == "password" {
            password = str_split[1]
        }
    }

    // return DSN
    // NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
    return fmt.Sprintf("%s:%s@tcp(127.0.0.1)/%s", username, password, dbName), nil // LOCAL & CLOUD VPS
    // return fmt.Sprintf("%s:%s@tcp(tools.db.svc.eqiad.wmflabs)/%s", username, password, dbName), nil // TOOLFORGE
}

// creates the DB if it doesn't already exist, and returns a connection to the DB
func DBConnection() (*sql.DB, error) {
    // PART 1: CREATE DB IF IT DOESN'T ALREADY EXIST

    // get DSN
    dbName, err := DSN("")
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return nil, err
    }

    // open DB
    db, err := sql.Open("mysql", dbName)
    if err != nil {
        log.Printf("Error %s when opening DB\n", err)
        return nil, err
    }

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // create DB if not exists
    // NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
    res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS wdp") // LOCAL & CLOUD VPS
    // res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS s54717__wdp_p") // TOOLFORGE
    if err != nil {
        log.Printf("Error %s when creating DB\n", err)
        return nil, err
    }

    // see how many rows affected (should be 0)
    no, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when fetching rows", err)
        return nil, err
    }
    log.Printf("rows affected %d\n", no)

    db.Close()

    // PART 2: CONNECT TO EXISTING DB

    // get DSN again, this time for the specific DB we just made
    // NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON TOOLFORGE VS LOCALLY
    dbName, err = DSN("wdp") // LOCAL & CLOUD VPS
    // dbName, err = DSN("s54717__wdp_p") // TOOLFORGE
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return nil, err
    }

    // open the DB
    db, err = sql.Open("mysql", dbName)
    if err != nil {
        log.Printf("Error %s when opening DB", err)
        return nil, err
    }

    // config stuff — TODO: this might have to go
    db.SetMaxOpenConns(60)
    // db.SetMaxIdleConns(30)
    db.SetMaxIdleConns(0)


    // set context
    ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // make sure connection works by pinging
    err = db.PingContext(ctx)
    if err != nil {
        log.Printf("Errors %s pinging DB", err)
        return nil, err
    }

    log.Printf("Connected to DB successfully\n")
    return db, nil
}

// creates a table with name tbl_name in DB db. Called in init_db.go.
func CreateTable(db *sql.DB, tbl_name string) error {
	var query string

    // check to make sure tbl_name is right format, then construct query based on type
	if strings.HasPrefix(tbl_name, "data_") {
    	query = `CREATE TABLE IF NOT EXISTS ` + tbl_name + `(id int primary key auto_increment, name text)`
    } else if strings.HasPrefix(tbl_name, "output_") {
    	query = `CREATE TABLE IF NOT EXISTS ` + tbl_name + `(Name text, Views int, Epsilon float, Delta float)`
    } else {
    	return fmt.Errorf("input to create table was not properly formated: %s", tbl_name)
    }

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // execute query and check rows affected (should be 0)
    res, err := db.ExecContext(ctx, query)
    if err != nil {
        log.Printf("Error %s when creating product table", err)
        return err
    }
    rows, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when getting rows affected", err)
        return err
    }

    log.Printf("Rows affected when creating table: %d\n", rows)
    return nil
}

// function for inserting faux data for a specific language in batches so as not
// to overwhelm the limits of mysql for loading data (which is around ~50,000)
func BatchInsert(db *sql.DB, tbl_name string, topFiftyArticles [50]Article) error {
    // initialize things to insert, batch counter, and query string
    var inserts []string
    var params []interface{}
    batch := 0
    query := "INSERT INTO " + tbl_name + "(name) VALUES "

    // for each of the top fifty articles
    for i := 0; i < 50; i++ {
        // for the number of views that it has
    	for j := 0; j < topFiftyArticles[i].Views; j++ {
            // append a parameterized variable to the query and the name of the page
    		inserts = append(inserts, "(?)")
    		page := strings.ReplaceAll(topFiftyArticles[i].Name, "'", "")
        	params = append(params, page)

            // increment the batch counter
            batch++

            // if the batch counter is 30,000 or greater
            if batch >= 30000 {

                // insert the values into the db
                err := insert(db, query, inserts, params)
                if err != nil {
                    log.Printf("error %s while inserting into table %s", err, tbl_name)
                }

                // reset everything back to 0/empty list
                inserts = nil
                params = nil
                batch = 0
            }
    	}
    }

    // insert whatever is left at the end
    err := insert(db, query, inserts, params)
    if err != nil {
        log.Printf("error %s while inserting", err)
    }

    return nil
}


// The actual workhorse of the inserion process. Safely inserts a set of params
// into a query and adds the whole thing to the database.
func insert(db *sql.DB, query string, inserts []string, params []interface{}) error {
    // create the query based on the insert list
    queryVals := strings.Join(inserts, ",")
    query = query + queryVals

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // prepare the query
    stmt, err := db.PrepareContext(ctx, query)
    if err != nil {
        log.Printf("Error %s when preparing SQL statement", err)
        return err
    }
    defer stmt.Close()

    // execute the query and see how many rows were affected
    res, err := stmt.ExecContext(ctx, params...)
    if err != nil {
        log.Printf("Error %s when inserting row into table", err)
        return err
    }
    rows, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when finding rows affected", err)
        return err
    }
    log.Printf("%d sessions created ", rows)
    return nil
}

// function to query the database and get back the normal count and a DP count
// based on the input (epsilon, delta) tuple
func Query(db *sql.DB, lang string, epsilon, delta float64) ([]TableRow, []TableRow, error) {
    // set up output structs
    var normalCount []TableRow
    var dpCount []TableRow

    // get the nam of the table we should be querying
    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
    var tbl_name = fmt.Sprintf("output_%s_%s", lang, yesterday)

    // create the query -- use the mysql round function to get around the fact that floats are imprecise
    var query = `SELECT * FROM ` + tbl_name + ` WHERE (Epsilon=-1 AND Delta=-1) OR (ROUND(Epsilon, 1)=ROUND(?, 1) AND ROUND(Delta, 9)=ROUND(?, 9))`

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // query the table (-1 is the code for normal, so we get -1 and the input epsilon and delta)
    res, err := db.QueryContext(ctx, query, epsilon, delta)
    // res, err := db.Query(query, epsilon, delta)
    // res, err := db.Query(query)
    if err != nil {
        log.Printf("Error %s when conducting query", err)
        return normalCount, dpCount, err
    }
    defer res.Close()
    

    // iterate through results
    for res.Next() {
        var row TableRow

        res.Scan(&row.Name, &row.Views, &row.Epsilon, &row.Delta)
        // log.Print(row)

        // if epsilon or delta is -1, add to the normal list; otherwise, add to the dpcount list
        if row.Epsilon == float64(-1) || row.Delta == float64(-1) {
            normalCount = append(normalCount, row)
        } else {
            dpCount = append(dpCount, row)
        }
    }

    return normalCount, dpCount, nil
}

// function for systematically dropping the tables of old data from previous days.
// called in clean_db.go, and should be used after beam.go does aggregating.
func DropOldData(db *sql.DB) error {
    // get the date of tables we should be keeping
    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")

    // get the names of all tables in the db
    res, err := db.Query("SHOW TABLES")
    if err != nil {
    	log.Printf("Error %s in showing tables query", err)
    	return err
    }

    var table string

    // iterate through the results
    for res.Next() {
        res.Scan(&table)
        // if the table is not from yesterday and has the prefix output, we drop it
        if !strings.HasSuffix(table, yesterday) && strings.HasPrefix(table, "output_") {
        	err := drop(db, table)
        	if err != nil {
        		log.Printf("Error %s while dropping table %s", err, table)
        		return err
        	}
        }
    }
    return nil
}

// function for systematically dropping the tables of faux data that we get as an
// input. called in clean_db.go, and should be used after beam.go does aggregating.
func DropSyntheticData(db *sql.DB) error {

    //get the names of all the tables in the db
    res, err := db.Query("SHOW TABLES")
    if err != nil {
    	log.Printf("Error %s in showing tables query", err)
    	return err
    }

    var table string

    // iterate through results
    for res.Next() {
        res.Scan(&table)
        // if the table has the prefix "data_", we drop it
        if strings.HasPrefix(table, "data_") {
        	err := drop(db, table)
        	if err != nil {
        		log.Printf("Error %s while dropping table %s", err, table)
        		return err
        	}
        }
    }
    return nil
}

// function for dropping a table named tbl_name from the DB. called in DropSyntheticData
// and DropOldData.
func drop(db *sql.DB, tbl_name string) error {
    // construct query
    query := "DROP TABLE " + tbl_name

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // execute the drop
    _, err := db.ExecContext(ctx, query)
    if err != nil {
        log.Printf("Error %s dropping table", err)
        return err
    }

    log.Printf("table %s dropped", tbl_name)
    return nil
}
