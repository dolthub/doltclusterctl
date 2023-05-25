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
	"crypto/tls"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDBStates(t *testing.T) {
	t.Run("EmptyState", func(t *testing.T) {
		res := LoadDBStates(context.Background(), &State{})
		assert.Len(t, res, 0)

	})
}

func TestRenderDSN(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		res := RenderDSN(&Config{}, "localhost", 3306)
		assert.Equal(t, "root@tcp(localhost:3306)/dolt_cluster", res)
	})
	t.Run("Username", func(t *testing.T) {
		oldval := os.Getenv("DOLT_USERNAME")
		defer os.Setenv("DOLT_USERNAME", oldval)
		os.Setenv("DOLT_USERNAME", "test_username")
		res := RenderDSN(&Config{}, "localhost", 3306)
		assert.Equal(t, "test_username@tcp(localhost:3306)/dolt_cluster", res)
	})
	t.Run("Password", func(t *testing.T) {
		oldval := os.Getenv("DOLT_PASSWORD")
		defer os.Setenv("DOLT_PASSWORD", oldval)
		os.Setenv("DOLT_PASSWORD", "test_password")
		res := RenderDSN(&Config{}, "localhost", 3306)
		assert.Equal(t, "root:test_password@tcp(localhost:3306)/dolt_cluster", res)
	})
	t.Run("UsernamePassword", func(t *testing.T) {
		olduser := os.Getenv("DOLT_USERNAME")
		defer os.Setenv("DOLT_USERNAME", olduser)
		oldpass := os.Getenv("DOLT_PASSWORD")
		defer os.Setenv("DOLT_PASSWORD", oldpass)
		os.Setenv("DOLT_USERNAME", "test_username")
		os.Setenv("DOLT_PASSWORD", "test_password")
		res := RenderDSN(&Config{}, "localhost", 3306)
		assert.Equal(t, "test_username:test_password@tcp(localhost:3306)/dolt_cluster", res)
	})
	t.Run("TLSInsecure", func(t *testing.T) {
		res := RenderDSN(&Config{TLSInsecure: true}, "localhost", 3306)
		assert.Equal(t, "root@tcp(localhost:3306)/dolt_cluster?tls=skip-verify", res)
	})
	t.Run("TLSVerified", func(t *testing.T) {
		res := RenderDSN(&Config{TLSVerified: true}, "localhost", 3306)
		assert.Equal(t, "root@tcp(localhost:3306)/dolt_cluster?tls=true", res)
	})
	t.Run("TLSConfig", func(t *testing.T) {
		res := RenderDSN(&Config{TLSConfig: &tls.Config{}}, "localhost", 3306)
		assert.Equal(t, "root@tcp(localhost:3306)/dolt_cluster?tls=custom", res)
	})
}
