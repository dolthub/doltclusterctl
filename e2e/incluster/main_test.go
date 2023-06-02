// Copyright 2023 DoltHub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func TestCreateSomeData(t *testing.T) {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = time.Second * 10
	err := backoff.Retry(func() error {
		db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s:3306)/", *DBHostname))
		if err != nil {
			return err
		}
		ctx := context.TODO()
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, "CREATE DATABASE testdata")
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, "USE testdata")
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, "CREATE TABLE vals (id INT PRIMARY KEY, val INT)")
		if err != nil {
			return err
		}
		_, err = conn.ExecContext(ctx, "INSERT INTO vals (id, val) VALUES (0,1),(1,1),(2,2),(3,3),(4,5),(5,8),(6,13),(7,21),(8,34),(9,55)")
		if err != nil {
			return backoff.Permanent(err)
		}
		return nil
	}, bo)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssertCreatedDataPresent(t *testing.T) {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = time.Second * 10
	err := backoff.Retry(func() error {
		db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s:3306)/testdata", *DBHostname))
		if err != nil {
			return err
		}
		row := db.QueryRowContext(context.TODO(), "SELECT COUNT(*) FROM vals")
		if row.Err() != nil {
			return row.Err()
		}
		var count int
		err = row.Scan(&count)
		if err != nil {
			return err
		}
		if count != 10 {
			return fmt.Errorf("expected count to be 10, but was %d", count)
		}
		return nil
	}, bo)
	if err != nil {
		t.Fatal(err)
	}
}
