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

import "context"

type Role int

const (
	RoleUnknown Role = 0
	RolePrimary Role = 1
	RoleStandby Role = 2
)

type Instance interface {
	// The name of the instance. A human-readable description which means
	// something to an operator familiar with the deployment and the
	// service registry.
	Name() string

	// The hostname (or dotted decimal IP address, or IPv6
	// colon-hexadecimal address) at which this instance's sql-server
	// instance can be connected to by the running doltclusterctl.
	Hostname() string

	// The port on which one can connect to this sql-server instance.
	Port() int

	// The current traffic role for this instance in the service registry.
	// This can be Primary, in which case the instance is intended to
	// receive primary traffic, Standby, in which case this instance is
	// intended to receive standby traffic, or Unknown, in which case this
	// instance is not intended to receive traffic currently.
	//
	// The service registry or deployment context should attempt to provide
	// an endpoint which routes write traffic to the unique primary
	// instance, routes read-only traffic to the standby instances, and
	// avoids routing trafifc to the instances with an unknown traffic
	// role.
	Role() Role

	// Mark this instance as wanting primary traffic.
	MarkRolePrimary(context.Context) error

	// Mark this instance as wanting standby traffic.
	MarkRoleStandby(context.Context) error

	// Mark this instance as not wanting primary or standby traffic.
	MarkRoleUnknown(context.Context) error

	// This should work in concert with the service registry or deployment
	// mechanism to restart this instance. Blocks until the instance
	// reports ready or available in the service registry. Further sanity
	// checks can be performed against the reported sql-server endpoint,
	// for example.
	//
	// After a restart, the instance state should reflect the most recent
	// state of the instance from the service registry and deployment
	// control plane.
	Restart(context.Context) error
}

type Cluster interface {
	// The name of the cluster deployment. A human-readable description
	// which means something to an operator familiar with the deployment
	// and the service registry.
	Name() string

	// The number of replicas in the cluster.
	NumReplicas() int

	// Accessor for each replica instance in the deployment. For a given
	// run on doltclusterctl, replicas are stably identified by an integer
	// |[0, NumReplicas())|.
	Instance(int) Instance
}
