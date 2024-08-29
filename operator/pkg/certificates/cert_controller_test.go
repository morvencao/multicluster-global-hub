// Copyright (c) Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project
// Licensed under the Apache License 2.0

package certificates

import (
	"context"
	"testing"
	"time"

	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stolostron/multicluster-global-hub/operator/api/operator/v1alpha4"
	"github.com/stolostron/multicluster-global-hub/pkg/constants"
)

func init() {
	s := scheme.Scheme
	v1alpha4.SchemeBuilder.AddToScheme(s)
}

func newDeployment(name string) *appv1.Deployment {
	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		Spec: appv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"label": "value"},
				},
			},
		},
		Status: appv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
}

func TestOnAdd(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	caSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              serverCACerts,
			Namespace:         namespace,
			CreationTimestamp: metav1.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC),
		},
	}
	onAdd(c)(caSecret)
	c = fake.NewClientBuilder().WithRuntimeObjects(newDeployment(constants.InventoryDeploymentName)).Build()
	onAdd(c)(caSecret)
	dep := &appv1.Deployment{}
	c.Get(context.TODO(),
		types.NamespacedName{Name: constants.InventoryDeploymentName, Namespace: namespace},
		dep)
	if dep.Spec.Template.ObjectMeta.Labels[restartLabel] == "" {
		t.Fatalf("Failed to inject restart label")
	}
}

func TestOnDelete(t *testing.T) {
	caSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverCACerts,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("new cert-"),
		},
	}
	deletCaSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverCACerts,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("old cert"),
		},
	}
	c := fake.NewClientBuilder().WithRuntimeObjects(caSecret, getMGH()).Build()
	onDelete(c)(deletCaSecret)
	c.Get(context.TODO(), types.NamespacedName{Name: serverCACerts, Namespace: namespace}, caSecret)
	data := string(caSecret.Data["tls.crt"])
	if data != "new cert-" {
		t.Fatalf("deleted cert not added back: %s", data)
	}
}

func TestOnUpdate(t *testing.T) {
	certSecret := getExpiredCertSecret()
	oldCertLength := len(certSecret.Data["tls.crt"])
	c := fake.NewClientBuilder().WithRuntimeObjects(certSecret).Build()
	onUpdate(c)(certSecret, certSecret)
	certSecret.Name = clientCACerts
	onUpdate(c)(certSecret, certSecret)
	certSecret.Name = guestCerts
	onUpdate(c)(certSecret, certSecret)
	certSecret.Name = serverCerts
	onUpdate(c)(certSecret, certSecret)
	c.Get(context.TODO(), types.NamespacedName{Name: serverCACerts, Namespace: namespace}, certSecret)
	if len(certSecret.Data["tls.crt"]) <= oldCertLength {
		t.Fatal("certificate not renewed correctly")
	}
}
