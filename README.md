doltclusterctl
==============

A small control plane job for interacting with a dolt cluster running within
kubernetes.

This binary perform certain operational tasks on the state of the sql-servers
and the labels for the pods in a dolt cluster.  Designed to be run within the
Kubernetes cluster with a service account that has permissions to list and edit
pods and StatefulSets.

Usage
-----

Generally run within a cluster using `kubectl`, and run with a `serviceAccount`
that has permissions to read StatefulSets and to read and edit pods.

For example, something like:

```sh
kubectl run -i --tty \
    --image localhost:5000/doltclusterctl:latest \
    --image-pull-policy Always \
    --restart=Never \
    --rm \
    --overrides '{"apiVersion": "v1","kind": "Pod","spec": {"serviceAccountName": "doltclusterctl"}}' \
    -n dolt \
    doltclusterctl -- \
    doltclusterctl -n dolt applyprimarylabels dolt
```

The `-n NAMESPACE` flag tells the binary which namespace the StatefulSet lives
in. The default is `default`.

The next parameter is the operation to run. The operations are:

- `applyprimarylabels`
- `gracefulfailover`
- `promotestandby`
- `rollingrestart`

The last parameter is the name of the stateful set on which to operate.

Operations
----------

`applyprimarylabels` is the simplest operation. It queries the state of the
dolt servers running within the stateful set and looks for the server instance
which is currently the primary. If it finds it, it applys appropriate labels to
the Pods in the StatefulSet so that traffic will be routed to the primary.

The traffic routing should be setup to route write traffic to the Pod which is
labeled with `dolthub.com/cluster_role=primary`.

`gracefulfailover` can be used to shift traffic away from the current primary. It:

1. Removes routing of traffic to the current primary.
2. Makes the current primary assume role standby.
3. Makes a different chosen standby server assume role primary.
4. Starts routing traffic to the new primary.

For a cluster with more than one standby, a useful option is
`-min-caughtup-standbys N`, which can be given to `gracefulfailover`. In that
case, `gracefulfailover` will succeed as long as at least `N` standbys can be
caught up. The standby which gets promoted to `primary` will be one of the
standbys which was successfully caught up.

`promotestandby` is more aggressive in its behavior. Without causing the
existing primary to assume role standby, it makes a server in the cluster which
is currently a standby into the new primary and begins routing traffic to it.
This should only be used if `gracefulfailover` cannot succeed because the
current primary or one of the standbys is currently down.

`rollingrestart` will perform a graceful rolling restart of all the Pods in the
StatefulSet. It will first identify every Pod which is a standby and will
delete it, relying on the ReplicaController to bring it back. Once it is back,
it will move on to the next Pod. Once all standby Pods have been restarted in
this way, it will perform a graceful failover of the primary. It will then
delete the old primary Pod, which is now a standby, and will wait for it come
back up.

`rollingrestart` should be run every time StatefulSet spec.template.spec.image:
is changed in order to perform a dolt upgrade. It can also be run in order to
pick up new config.yaml settings across the cluster, for example.

Authentication
--------------

Set the environment variables `DOLT_USERNAME` and `DOLT_PASSWORD` to control
the credentials the tool uses to connect to the sql-server instances. By
default, it uses `root` with no password.

TODO
====

* There should be a way of manually selecting a replica to become a primary in the case of something like `detected_broken_config`.
