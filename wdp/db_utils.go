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
    // NOTE: SWITCH WHICH OF THESE STATEMENTS IS COMMENTED OUT TO RUN ON CLOUD VPS VS LOCALLY
    // f, err := os.Open("/Users/haltriedman/replica.my.cnf") // LOCAL
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
    return fmt.Sprintf("%s:%s@tcp(127.0.0.1)/%s", username, password, dbName), nil
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
    res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS wdp")
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
    dbName, err = DSN("wdp")
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
    if tbl_name == "data" {
        query = `CREATE TABLE IF NOT EXISTS data(pv_id INT PRIMARY KEY AUTO_INCREMENT, user_id TEXT, day DATE, lang TEXT, name TEXT)`
    } else if tbl_name == "output" {
        query = `CREATE TABLE IF NOT EXISTS output(Name TEXT, Views INT, Lang TEXT, Day DATE, Kind TEXT, Epsilon FLOAT, Delta FLOAT)`
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
// to overwhelm the limits of mysql for loading data (which is around ~50,000 placeholders)
func BatchInsert(db *sql.DB, tbl_name, date, lang string, topFiftyArticles [50]Article) error {
    // initialize things to insert, batch counter, and query string
    var inserts []string
    var params []interface{}
    batch := 0
    query := "INSERT INTO " + tbl_name + "(user_id, day, lang, name) VALUES "

    // for each of the top fifty articles
    for i := 0; i < 50; i++ {
        // for the number of views that it has
        for j := 0; j < topFiftyArticles[i].Views; j++ {
            // append a parameterized variable to the query and the name of the page
            inserts = append(inserts, "(?, ?, ?, ?)")
            params = append(params, "a", date, lang, topFiftyArticles[i].Name)

            // increment the batch counter
            batch++

            // if the batch counter is 10,000 or greater
            if batch >= 10000 {

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

    // create the query -- use the mysql round function to get around the fact that floats are imprecise
    // the inner join filters to just the most recent day of data, and lang filters the language
    // -1 is the code for normal, so we get -1 and the input epsilon and delta
    var query = `
        SELECT Name, Views, Lang, Day, Kind, Epsilon, Delta FROM output
        INNER JOIN (
            SELECT MAX(Day) as max_day
            FROM output
        ) sub
        ON output.Day=sub.max_day
        WHERE
            ((Epsilon=-1 AND Delta=-1) OR (ROUND(Epsilon, 1)=ROUND(?, 1) AND ROUND(Delta, 9)=ROUND(?, 9)))
            AND Lang=?
            And Kind="pv"
        `

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // query the table
    res, err := db.QueryContext(ctx, query, epsilon, delta, lang)
    if err != nil {
        log.Printf("Error %s when conducting query", err)
        return normalCount, dpCount, err
    }
    defer res.Close()
    

    // iterate through results
    for res.Next() {
        var row TableRow

        res.Scan(&row.Name, &row.Views, &row.Lang, &row.Day, &row.Kind, &row.Epsilon, &row.Delta)

        // if epsilon or delta is -1, add to the normal list; otherwise, add to the dpcount list
        if row.Epsilon == float64(-1) || row.Delta == float64(-1) {
            normalCount = append(normalCount, row)
        } else {
            dpCount = append(dpCount, row)
        }
    }

    return normalCount, dpCount, nil
}

// function for systematically dropping the rows of old data from previous days.
// called in clean_db.go, and should be used after beam.go does aggregating.
func DropOldData(db *sql.DB, tbl_name, date string) error {
    if tbl_name != "data" && tbl_name != "output" {
        return fmt.Errorf("Error: incorrect formatting for table name %s", tbl_name)
    }

    query := `DELETE FROM ` + tbl_name + ` WHERE day != "` + date + `"`

    // set context
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    // execute the row deletion
    res, err := db.ExecContext(ctx, query)
    if err != nil {
        log.Printf("Error %s deleting rows", err)
        return err
    }

    rows, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when finding rows affected", err)
        return err
    }
    log.Printf("%d pageviews deleted ", rows)
    return nil
}

