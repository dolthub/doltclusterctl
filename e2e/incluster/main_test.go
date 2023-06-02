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
