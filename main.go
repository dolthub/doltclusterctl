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
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	var cfg Config
	cfg.Parse(flag.CommandLine, os.Args[1:])

	ctx, f := context.WithDeadline(context.Background(), time.Now().Add(cfg.Timeout))
	defer f()

	if cfg.TLSConfig != nil {
		mysql.RegisterTLSConfig("custom", cfg.TLSConfig)
	}

	log.Printf("running %s against %s/%s", cfg.CommandStr, cfg.Namespace, cfg.StatefulSetName)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("could not load kubernetes InClusterConfig: %v", err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("could not build kubernetes client for config: %v", err.Error())
	}

	cluster, err := NewKubernetesCluster(ctx, cfg.Namespace, cfg.StatefulSetName, clientset)
	if err != nil {
		log.Fatalf("could not load stateful set %s/%s and its pods: %v", cfg.Namespace, cfg.StatefulSetName, err.Error())
	}

	if err := cfg.Command.Run(ctx, &cfg, cluster); err != nil {
		log.Fatalf("error running command: %v", err.Error())
	}
}
