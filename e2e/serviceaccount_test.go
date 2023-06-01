package main

import (
	"context"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func CreateDoltClusterCtlServiceAccount(ctx context.Context, c *envconf.Config) (context.Context, error) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "doltclusterctl", Namespace: c.Namespace()},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "update", "list", "watch", "delete"},
		}, {
			APIGroups: []string{"apps"},
			Resources: []string{"statefulsets"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}
	serviceaccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "doltclusterctl", Namespace: c.Namespace()},
	}
	rolebinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "doltclusterctl", Namespace: c.Namespace()},
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: "doltclusterctl",
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "doltclusterctl",
		},
	}

	client, err := c.NewClient()
	if err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, role); err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, serviceaccount); err != nil {
		return ctx, err
	}
	if err := client.Resources().Create(ctx, rolebinding); err != nil {
		return ctx, err
	}

	return ctx, nil
}
