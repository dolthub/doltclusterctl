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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

const SubcommandsUsage = `
SUBCOMMANDS

  doltclusterctl applyprimarylabels statefulset_name - sets/unsets the primary labels on the pods in the StatefulSet with metadata.name: statefulset-name; labels the other pods standby.
  doltclusterctl gracefulfailover statefulset_name - takes the current primary, marks it as a standby, and marks the next replica in the set as the primary.
  doltclusterctl promotestandby statefulset_name - takes the first reachable standby and makes it the new primary.
  doltclusterctl rollingrestart statefulset_name - deletes all pods in the stateful set, one at a time, waiting for the deleted pods to be recreated and ready before moving on; gracefully fails over the primary before deleting it.
`

type Config struct {
	// The kubernetes namespace of the statefulset.
	Namespace string

	// A *tls.Config which could have been built in argument parsing.
	TLSConfig *tls.Config

	// Use required TLS verified mode with default settings.
	TLSVerified bool
	// Use required TLS mode with default settings, but the endpoint is
	// unverified.
	TLSInsecure bool

	// The timeout for the entire command run.
	Timeout time.Duration
	// A timeout for how long to wait for each individual restarted pod to
	// come back and be ready.
	WaitForReady time.Duration

	CommandStr      string
	StatefulSetName string
	Command         Command

	// The number of standbys which must be caught up, when running a
	// graceful failover, in order to proceed.
	MinCaughtUpStandbys int
}

func (c *Config) InitFlagSet(set *flag.FlagSet) {
	set.StringVar(&c.Namespace, "n", "default", "namespace of the stateful set to operate on")

	set.IntVar(&c.MinCaughtUpStandbys, "min-caughtup-standbys", -1, "the number of standby servers which must be caughtup on a graceful failover in order to succeed")

	set.Func("tls-server-name", "if provided, enables manadatory verified TLS mode and overrides the server name to verify as the CN or SAN of the leaf certificate (and present in SNI)", func(sn string) error {
		if c.TLSInsecure {
			return errors.New("cannot provide -tls-server-name with -tls-insecure")
		}
		if c.TLSConfig == nil {
			c.TLSConfig = &tls.Config{}
		}
		c.TLSConfig.ServerName = sn
		return nil
	})
	set.Func("tls-ca", "if provided, enables mandatory verified TLS mode; provides the path to a file to use as the certificate authority roots for verifying the server certificate", func(path string) error {
		if c.TLSInsecure {
			return errors.New("cannot provide -tls-ca with -tls-insecure")
		}
		rootCertPool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read tls-ca %s: %w", path, err)
		}
		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			return fmt.Errorf("failed to append PEM from %s", path)
		}
		if c.TLSConfig == nil {
			c.TLSConfig = &tls.Config{}
		}
		c.TLSConfig.RootCAs = rootCertPool
		return nil
	})
	set.Var((*tlsVerifiedFlagValue)(c), "tls", "if provided, enables manadatory verified TLS mode")
	set.Var((*tlsInsecureFlagValue)(c), "tls-insecure", "if true, enables tls mode for communicating with the server, but does not verify the server's certificate")

	set.DurationVar(&c.Timeout, "timeout", time.Second*30, "the number of seconds the entire command has to run before it timeouts and exits non-zero")
	set.DurationVar(&c.WaitForReady, "wait-for-ready", time.Second*120, "the number of seconds to wait for a single pod to become ready when performing a rollingrestart until we consider the operation failed")

	set.Usage = func() {
		fmt.Fprintf(set.Output(), "Usage of %s:\n\n", set.Name())
		fmt.Fprintf(set.Output(), "  %s [COMMON OPTIONS...] subcommand statefulset_name\n\nCOMMON OPTIONS\n\n", set.Name())
		flag.PrintDefaults()
		fmt.Fprint(set.Output(), SubcommandsUsage)
	}
}

func (c *Config) Parse(set *flag.FlagSet, args []string) error {
	c.InitFlagSet(set)
	err := set.Parse(args)
	if err != nil {
		return err
	}

	errF := func(err error) error {
		switch set.ErrorHandling() {
		case flag.ContinueOnError:
			return err
		case flag.ExitOnError:
			os.Exit(2)
		case flag.PanicOnError:
			panic(err)
		}
		panic("unexpected ErrorHandling value")
	}

	if set.NArg() != 2 {
		str := fmt.Sprintf("must provide subcommand and the name of the StatefulSet")
		fmt.Fprintln(set.Output(), str)
		set.Usage()
		return errF(errors.New(str))
	}

	c.CommandStr = set.Arg(0)
	c.StatefulSetName = set.Arg(1)

	if c.CommandStr == "applyprimarylabels" {
		c.Command = ApplyPrimaryLabels{}
	} else if c.CommandStr == "gracefulfailover" {
		c.Command = GracefulFailover{}
	} else if c.CommandStr == "promotestandby" {
		c.Command = PromoteStandby{}
	} else if c.CommandStr == "rollingrestart" {
		c.Command = RollingRestart{}
	} else {
		str := fmt.Sprintf("did not find subcommand %s", c.CommandStr)
		fmt.Fprintln(set.Output(), str)
		set.Usage()
		return errF(errors.New(str))
	}

	return nil
}

type tlsVerifiedFlagValue Config

func (v *tlsVerifiedFlagValue) Set(s string) error {
	pv, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("cannot parse %s as bool for flag", s)
	}
	if pv && v.TLSInsecure {
		return errors.New("cannot provide -tls-insecure and -tls-verified")
	}
	v.TLSVerified = pv
	return nil
}

func (v *tlsVerifiedFlagValue) String() string {
	return strconv.FormatBool(v.TLSVerified)
}

func (v *tlsVerifiedFlagValue) IsBoolFlag() bool {
	return true
}

type tlsInsecureFlagValue Config

func (v *tlsInsecureFlagValue) Set(s string) error {
	pv, err := strconv.ParseBool(s)
	if err != nil {
		return fmt.Errorf("cannot parse %s as bool for flag", s)
	}
	if pv && v.TLSVerified {
		return errors.New("cannot provide -tls-insecure and -tls-verified")
	}
	if pv && v.TLSConfig != nil && v.TLSConfig.ServerName != "" {
		return errors.New("cannot provide -tls-insecure and -tls-server-name")
	}
	if pv && v.TLSConfig != nil && v.TLSConfig.RootCAs != nil {
		return errors.New("cannot provide -tls-insecure and -tls-ca")
	}
	v.TLSInsecure = pv
	return nil
}

func (v *tlsInsecureFlagValue) String() string {
	return strconv.FormatBool(v.TLSInsecure)
}

func (v *tlsInsecureFlagValue) IsBoolFlag() bool {
	return true
}
