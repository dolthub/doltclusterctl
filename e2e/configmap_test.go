package main

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var ConfigMapKey *struct{}

func WithConfigMap(ctx context.Context, cm *v1.ConfigMap) context.Context {
	return context.WithValue(ctx, &ConfigMapKey, cm)
}

func GetConfigMap(ctx context.Context) (*v1.ConfigMap, bool) {
	if v := ctx.Value(&ConfigMapKey); v != nil {
		return v.(*v1.ConfigMap), true
	} else {
		return nil, false
	}
}

func DeleteConfigMap(ctx context.Context, c *envconf.Config) (context.Context, error) {
	if cm, ok := GetConfigMap(ctx); ok {
		client, err := c.NewClient()
		if err != nil {
			return ctx, err
		}
		err = client.Resources().Delete(ctx, cm)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func CreateConfigMap(ctx context.Context, c *envconf.Config) (context.Context, error) {
	cm := NewConfigMap(c.Namespace())

	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	err = client.Resources().Create(ctx, cm)
	if err != nil {
		return ctx, err
	}
	return WithConfigMap(ctx, cm), nil
}

func NewConfigMap(namespace string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "dolt", Namespace: namespace},
		Data: map[string]string{
			"dolt-0.yaml": `
log_level: trace
cluster:
  standby_remotes:
    - name: dolt-1
      remote_url_template: http://dolt-1.dolt-internal:50051/{database}
  bootstrap_epoch: 1
  bootstrap_role: primary
  remotesapi:
    port: 50051
listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128
`,
			"dolt-1.yaml": `
log_level: trace
cluster:
  standby_remotes:
    - name: dolt-0
      remote_url_template: http://dolt-0.dolt-internal:50051/{database}
  bootstrap_epoch: 1
  bootstrap_role: standby
  remotesapi:
    port: 50051
listener:
  host: 0.0.0.0
  port: 3306
  max_connections: 128
`,
		},
	}
}

// We do not want DNS caching while we run our tests.
func PatchCoreDNSConfigMap(ctx context.Context, c *envconf.Config) (context.Context, error) {
	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	var cm v1.ConfigMap
	err = client.Resources().Get(ctx, "coredns", "kube-system", &cm)
	if err != nil {
		return ctx, err
	}
	contents := cm.Data["Corefile"]
	contents = strings.ReplaceAll(contents, "    cache 30", "")
	cm.Data["Corefile"] = contents
	err = client.Resources().Update(ctx, &cm)
	if err != nil {
		return ctx, err
	}
	return ctx, nil
}
