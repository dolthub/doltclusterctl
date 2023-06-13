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
	"errors"
	"fmt"
	"net/url"
	"os"
)

func OpenDB(ctx context.Context, cfg *Config, instance Instance) (*sql.DB, error) {
	hostname := instance.Hostname()
	port := instance.Port()
	dsn := RenderDSN(cfg, hostname, port)
	return sql.Open("mysql", dsn)
}

func RenderDSN(cfg *Config, hostname string, port int) string {
	user := os.Getenv("DOLT_USERNAME")
	if user == "" {
		user = "root"
	}
	authority := user
	pass := os.Getenv("DOLT_PASSWORD")
	if pass != "" {
		authority += ":" + pass
	}

	params := make(url.Values)
	params["parseTime"] = []string{"true"}
	if cfg.TLSInsecure {
		params["tls"] = []string{"skip-verify"}
	} else if cfg.TLSConfig != nil {
		// TODO: This is spookily coupled to the config name in main
		params["tls"] = []string{"custom"}
	} else if cfg.TLSVerified {
		params["tls"] = []string{"true"}
	}
	return fmt.Sprintf("%s@tcp(%s:%d)/dolt_cluster?%s", authority, hostname, port, params.Encode())
}

func CallAssumeRole(ctx context.Context, cfg *Config, instance Instance, role string, epoch int) error {
	db, err := OpenDB(ctx, cfg, instance)
	if err != nil {
		return err
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	var status int

	q := fmt.Sprintf("CALL DOLT_ASSUME_CLUSTER_ROLE('%s', %d)", role, epoch)
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&status)
		if err != nil {
			return err
		}
		if status != 0 {
			return fmt.Errorf("result from call dolt_assume_role('%s', %d) was %d, not 0", role, epoch, status)
		}
	}
	if rows.Err() != nil {
		return rows.Err()
	}

	return nil
}

func LoadDBState(ctx context.Context, cfg *Config, instance Instance) DBState {
	errf := func(err error) error {
		return fmt.Errorf("error loading role and epoch for %s: %w", instance.Name(), err)
	}

	db, err := OpenDB(ctx, cfg, instance)
	if err != nil {
		return DBState{"", 0, instance, nil, errf(err)}
	}
	defer db.Close()

	conn, err := db.Conn(ctx)
	if err != nil {
		return DBState{"", 0, instance, nil, errf(err)}
	}
	defer conn.Close()

	role, epoch, err := loadRoleAndEpoch(ctx, conn)
	if err != nil {
		return DBState{"", 0, instance, nil, errf(err)}
	}

	state := DBState{role, epoch, instance, nil, nil}

	loadStatusRows(ctx, conn, &state)

	return state
}

func loadStatusRows(ctx context.Context, conn *sql.Conn, state *DBState) {
	rows, err := conn.QueryContext(ctx, "SELECT `database`, role, epoch, standby_remote, replication_lag_millis, last_update, current_error FROM `dolt_cluster`.`dolt_cluster_status`")
	if err != nil {
		state.Err = fmt.Errorf("error loading dolt_cluster_status table: %w", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status StatusRow
		err = rows.Scan(&status.Database, &status.Role, &status.Epoch, &status.Remote, &status.ReplicationLag, &status.LastUpdate, &status.CurrentError)
		if err != nil {
			state.Err = fmt.Errorf("error scanning status row: %w", err)
			return
		}
		state.Status = append(state.Status, status)
	}
	if rows.Err() != nil {
		state.Err = fmt.Errorf("error loading dolt_cluster_status table: %w", err)
		return
	}
}

func loadRoleAndEpoch(ctx context.Context, conn *sql.Conn) (string, int, error) {
	var role string
	var epoch int

	rows, err := conn.QueryContext(ctx, "SELECT @@global.dolt_cluster_role, @@global.dolt_cluster_role_epoch")
	if err != nil {
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&role, &epoch)
		if err != nil {
			return "", 0, err
		}
	} else if rows.Err() == nil {
		return "", 0, errors.New("querying cluster_role and epoch should have return values, but did not")
	}
	if rows.Err() != nil {
		return "", 0, rows.Err()
	}

	return role, epoch, nil
}

type StatusRow struct {
	Database       string
	Role           string
	Epoch          int
	Remote         string
	ReplicationLag sql.NullInt64
	LastUpdate     sql.NullTime
	CurrentError   sql.NullString
}

type DBState struct {
	Role     string
	Epoch    int
	Instance Instance
	Status   []StatusRow
	Err      error
}

func LoadDBStates(ctx context.Context, cfg *Config, cluster Cluster) []DBState {
	ret := make([]DBState, cluster.NumReplicas())
	for i := 0; i < cluster.NumReplicas(); i++ {
		instance := cluster.Instance(i)
		ret[i] = LoadDBState(ctx, cfg, instance)
	}
	return ret
}

// Returns the current valid primary based on the dbstates. Returns an error if
// there is no primary or if there is more than one primary.
func CurrentPrimaryAndEpoch(dbstates []DBState) (int, int, error) {
	highestepoch := 0
	currentprimary := -1
	for i := range dbstates {
		if dbstates[i].Role == "primary" {
			if currentprimary != -1 {
				return -1, -1, fmt.Errorf("more than one reachable pod was in role primary: %s and %s", dbstates[currentprimary].Instance.Name(), dbstates[i].Instance.Name())
			}
			currentprimary = i
		}
		if dbstates[i].Epoch > highestepoch {
			highestepoch = dbstates[i].Epoch
		}
	}

	if currentprimary == -1 {
		return -1, -1, errors.New("no reachable pod was in role primary")
	}

	return currentprimary, highestepoch, nil
}
