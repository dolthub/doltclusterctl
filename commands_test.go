// Copyright 2022 DoltHub, Inc.
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
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPickNextPrimary(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		res := PickNextPrimary([]DBState{})
		assert.Equal(t, -1, res)
	})
	t.Run("SingleStandby", func(t *testing.T) {
		res := PickNextPrimary([]DBState{{
			Role:  "primary",
			Epoch: 10,
		}, {
			Role:  "standby",
			Epoch: 10,
		}})
		assert.Equal(t, 1, res)
	})
	t.Run("TwoStandbys", func(t *testing.T) {
		t.Run("NoStatuses", func(t *testing.T) {
			res := PickNextPrimary([]DBState{{
				Role:  "standby",
				Epoch: 10,
			}, {
				Role:  "standby",
				Epoch: 10,
			}, {
				Role:  "primary",
				Epoch: 10,
			}})
			assert.Equal(t, 0, res)
		})
		earlierUpdateTime := time.Now().Add(-1 * time.Minute)
		laterUpdateTime := earlierUpdateTime.Add(1 * time.Minute)
		t.Run("NewestLastUpdated", func(t *testing.T) {
			t.Run("ComesSecond", func(t *testing.T) {
				res := PickNextPrimary([]DBState{{
					Role:  "standby",
					Epoch: 10,
					Status: []StatusRow{{
						Database: "one",
						LastUpdate: sql.NullTime{
							Valid: true,
							Time:  earlierUpdateTime,
						},
					}},
				}, {
					Role:  "standby",
					Epoch: 10,
					Status: []StatusRow{{
						Database: "one",
						LastUpdate: sql.NullTime{
							Valid: true,
							Time:  laterUpdateTime,
						},
					}},
				}, {
					Role:  "primary",
					Epoch: 10,
				}})
				assert.Equal(t, 1, res)
			})
			t.Run("ComesFirst", func(t *testing.T) {
				res := PickNextPrimary([]DBState{{
					Role:  "standby",
					Epoch: 10,
					Status: []StatusRow{{
						Database: "one",
						LastUpdate: sql.NullTime{
							Valid: true,
							Time:  laterUpdateTime,
						},
					}},
				}, {
					Role:  "standby",
					Epoch: 10,
					Status: []StatusRow{{
						Database: "one",
						LastUpdate: sql.NullTime{
							Valid: true,
							Time:  earlierUpdateTime,
						},
					}},
				}, {
					Role:  "primary",
					Epoch: 10,
				}})
				assert.Equal(t, 0, res)
			})
		})
	})
}
