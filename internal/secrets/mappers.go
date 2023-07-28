package secrets

import (
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func CheckSecretForQdb(obj client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	qdbName := obj.GetAnnotations()[crdv1beta1.AnnotationQuestDBName]
	if qdbName != "" {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      qdbName,
				Namespace: obj.GetNamespace(),
			},
		})
	}

	return requests

}
