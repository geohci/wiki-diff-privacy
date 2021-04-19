package wdp

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"context"
	"log"
	"time"
	"fmt"
	"strings"
    "github.com/joho/godotenv"
    "os"
)

func DSN(dbName string) (string, error) {
    err := godotenv.Load(".env")
    if err != nil {
        log.Printf("Error %s while loading .env\n", err)
        return "", err
    }
    username := os.Getenv("USERNAME")
    password := os.Getenv("PASSWORD")
    hostname := os.Getenv("HOSTNAME")
    return fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, hostname, dbName), nil
}

func DBConnection() (*sql.DB, error) {
    dbName, err := DSN("")
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return nil, err
    }
    db, err := sql.Open("mysql", dbName)
    if err != nil {
        log.Printf("Error %s when opening DB\n", err)
        return nil, err
    }

    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()
    res, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS wdp")
    if err != nil {
        log.Printf("Error %s when creating DB\n", err)
        return nil, err
    }
    no, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when fetching rows", err)
        return nil, err
    }
    log.Print("rows affected %d\n", no)

    db.Close()

    dbName, err = DSN("wdp")
    if err != nil {
    	log.Printf("Error %s when getting dbname\n", err)
    	return nil, err
    }
    db, err = sql.Open("mysql", dbName)
    if err != nil {
        log.Printf("Error %s when opening DB", err)
        return nil, err
    }
    //defer db.Close()

    db.SetMaxOpenConns(20)
    db.SetMaxIdleConns(20)

    ctx, cancelfunc = context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()
    err = db.PingContext(ctx)
    if err != nil {
        log.Printf("Errors %s pinging DB", err)
        return nil, err
    }
    log.Printf("Connected to DB %s successfully\n", dbName)
    return db, nil
}

func CreateTable(db *sql.DB, tbl_name string) error {
	var query string
	if strings.HasPrefix(tbl_name, "data_") {
    	query = `CREATE TABLE IF NOT EXISTS ` + tbl_name + `(id int primary key auto_increment, name text)`
    } else if strings.HasPrefix(tbl_name, "output_") {
    	query = `CREATE TABLE IF NOT EXISTS ` + tbl_name + `(page text, views int)`
    } else {
    	return fmt.Errorf("input to create table was not properly formated: %s", tbl_name)
    }
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()
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
    log.Printf("Rows affected when creating table: %d", rows)
    return nil
}

func BatchInsert(db *sql.DB, tbl_name string, topFiftyArticles [50]Article) error {
    var inserts []string
    var params []interface{}

    batch := 0
    query := "INSERT INTO " + tbl_name + "(name) VALUES "

    for i := 0; i < 50; i++ {
    	for j := 0; j < topFiftyArticles[i].Views; j++ {
    		inserts = append(inserts, "(?)")
    		page := strings.ReplaceAll(topFiftyArticles[i].Name, "'", "")
        	params = append(params, page)
            batch++
            if batch >= 30000 {
                err := insert(db, query, inserts, params)
                if err != nil {
                    log.Printf("error %s while inserting into table %s", err, tbl_name)
                }
                inserts = nil
                params = nil
                batch = 0
            }
    	}
    }

    err := insert(db, query, inserts, params)
    if err != nil {
        log.Printf("error %s while inserting", err)
    }

    return nil
}

func insert(db *sql.DB, query string, inserts []string, params []interface{}) error {
    queryVals := strings.Join(inserts, ",")
    query = query + queryVals

    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()
    stmt, err := db.PrepareContext(ctx, query)
    if err != nil {
        log.Printf("Error %s when preparing SQL statement", err)
        return err
    }
    defer stmt.Close()

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

func drop(db *sql.DB, tbl_name string) error {
    query := "DROP TABLE " + tbl_name
    ctx, cancelfunc := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelfunc()

    res, err := db.ExecContext(ctx, query)
    if err != nil {
        log.Printf("Error %s dropping table", err)
        return err
    }
    rows, err := res.RowsAffected()
    if err != nil {
        log.Printf("Error %s when finding rows affected", err)
        return err
    }

    log.Printf("%d sessions dropped in table %s", rows, tbl_name)
    return nil
}

func Query(lang string, epsilon, delta float64) {

}

func DropOldData(db *sql.DB) error {
    var yesterday = time.Now().AddDate(0, 0, -1).Format("2006_01_02")
    res, err := db.Query("SHOW TABLES")
    if err != nil {
    	log.Printf("Error %s in showing tables query", err)
    	return err
    }

    var table string

    for res.Next() {
        res.Scan(&table)
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


func DropSyntheticData(db *sql.DB) error {
    res, err := db.Query("SHOW TABLES")
    if err != nil {
    	log.Printf("Error %s in showing tables query", err)
    	return err
    }

    var table string

    for res.Next() {
        res.Scan(&table)
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
