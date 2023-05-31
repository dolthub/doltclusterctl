package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	_ "github.com/go-sql-driver/mysql"
)

var DBHostname = flag.String("dbhostname", "dolt", "the database hostname")

func TestMain(t *testing.M) {
	os.Exit(t.Run())
}

func TestConnectToService(t *testing.T) {
	// Run a simple test where we connect to the running dolt service.
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = time.Second * 10
	err := backoff.Retry(func() error {
		db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s:3306)/", *DBHostname))
		if err != nil {
			return err
		}
		err = db.PingContext(context.TODO())
		if err != nil {
			return err
		}
		return nil
	}, bo)
	if err != nil {
		t.Fatal(err)
	}
}
