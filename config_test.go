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
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigParse(t *testing.T) {
	t.Run("NoArgs", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{})
		assert.Error(t, err)
	})
	t.Run("OneArg", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"gracefulfailover"})
		assert.Error(t, err)
	})
	t.Run("ThreeArgs", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"gracefulfailover", "doltdb", "thisisunrecognized"})
		assert.Error(t, err)
	})
	t.Run("GracefulFailover", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"gracefulfailover", "doltdb"})
		assert.NoError(t, err)
	})
	t.Run("PromoteStandby", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"promotestandby", "doltdb"})
		assert.NoError(t, err)
	})
	t.Run("ApplyPrimaryLabels", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"applyprimarylabels", "doltdb"})
		assert.NoError(t, err)
	})
	t.Run("RollingRestart", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"rollingrestart", "doltdb"})
		assert.NoError(t, err)
	})
	t.Run("UnrecognizedCommand", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"doltclusterctlcannotdothis", "doltdb"})
		assert.Error(t, err)
	})
	t.Run("UnrecognizedFlag", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		err := cfg.Parse(&set, []string{"-doesnotexist", "rollingrestart", "doltdb"})
		assert.Error(t, err)
	})
}

func TestConfigFlagSet(t *testing.T) {
	t.Run("TLSInsecure", func(t *testing.T) {
		t.Run("Alone", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-insecure"})
			assert.NoError(t, err)
			assert.True(t, cfg.TLSInsecure)
		})
		t.Run("ExcludesTLSVerified", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls", "-tls-insecure"})
			assert.Error(t, err)
		})
		t.Run("ExcludesTLSServerName", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-server-name", "doltdb-0.doltdb", "-tls-insecure"})
			assert.Error(t, err)
		})
		t.Run("ExcludesTLSRootCA", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/validroots.pem", "-tls-insecure"})
			assert.Error(t, err)
		})
		t.Run("BadBool", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-insecure=WAT"})
			assert.Error(t, err)
		})
	})
	t.Run("TLSVerified", func(t *testing.T) {
		t.Run("Alone", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls"})
			assert.NoError(t, err)
			assert.True(t, cfg.TLSVerified)
		})
		t.Run("ExcludesTLSInsecure", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-insecure", "-tls"})
			assert.Error(t, err)
		})
		t.Run("BadBool", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls=WAT"})
			assert.Error(t, err)
		})
	})
	t.Run("Namespace", func(t *testing.T) {
		t.Run("Default", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{})
			assert.NoError(t, err)
			assert.Equal(t, "default", cfg.Namespace)
		})
		t.Run("Arg", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-n", "doltdb"})
			assert.NoError(t, err)
			assert.Equal(t, "doltdb", cfg.Namespace)
		})
	})
	t.Run("TLSRootCA", func(t *testing.T) {
		t.Run("Alone", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/validroots.pem"})
			assert.NoError(t, err)
			if assert.NotNil(t, cfg.TLSConfig) {
				assert.NotNil(t, cfg.TLSConfig.RootCAs)
			}
		})
		t.Run("WithTLSServerName", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-server-name", "doltdb-0.doltdb", "-tls-ca", "testdata/validroots.pem"})
			assert.NoError(t, err)
			if assert.NotNil(t, cfg.TLSConfig) {
				assert.NotNil(t, cfg.TLSConfig.RootCAs)
				assert.Equal(t, "doltdb-0.doltdb", cfg.TLSConfig.ServerName)
			}
		})
		t.Run("ExcludesTLSInsecure", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-insecure", "-tls-ca", "testdata/validroots.pem"})
			assert.Error(t, err)
		})
		t.Run("NonExistantFile", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/doesnotexist.pem"})
			assert.Error(t, err)
		})
		t.Run("InvalidCertInFile", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/invalidroots.pem"})
			assert.Error(t, err)
		})
		t.Run("NoCertsInFile", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/nopemdata.pem"})
			assert.Error(t, err)
		})
	})
	t.Run("TLSServerName", func(t *testing.T) {
		t.Run("Alone", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-server-name", "doltdb-0.doltdb"})
			assert.NoError(t, err)
			if assert.NotNil(t, cfg.TLSConfig) {
				assert.Equal(t, "doltdb-0.doltdb", cfg.TLSConfig.ServerName)
			}
		})
		t.Run("WithTLSServerName", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-ca", "testdata/validroots.pem", "-tls-server-name", "doltdb-0.doltdb"})
			assert.NoError(t, err)
			if assert.NotNil(t, cfg.TLSConfig) {
				assert.NotNil(t, cfg.TLSConfig.RootCAs)
				assert.Equal(t, "doltdb-0.doltdb", cfg.TLSConfig.ServerName)
			}
		})
		t.Run("ExcludesTLSInsecure", func(t *testing.T) {
			var cfg Config
			var set flag.FlagSet
			cfg.InitFlagSet(&set)
			err := set.Parse([]string{"-tls-insecure", "-tls-server-name", "doltdb-0.doltdb"})
			assert.Error(t, err)
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		cfg.InitFlagSet(&set)
		err := set.Parse([]string{"-timeout", "5m"})
		assert.NoError(t, err)
		assert.Equal(t, 5*time.Minute, cfg.Timeout)
	})
	t.Run("WaitForReady", func(t *testing.T) {
		var cfg Config
		var set flag.FlagSet
		cfg.InitFlagSet(&set)
		err := set.Parse([]string{"-wait-for-ready", "1h"})
		assert.NoError(t, err)
		assert.Equal(t, time.Hour, cfg.WaitForReady)
	})
}
