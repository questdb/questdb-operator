package secrets

import (
	"context"
	"fmt"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type QuestDBSecrets struct {
	IlpSecret  *v1.Secret
	PsqlSecret *v1.Secret
}

func GetSecrets(ctx context.Context, c client.Client, q *crdv1beta1.QuestDB) (QuestDBSecrets, error) {
	var (
		err     error
		secrets = QuestDBSecrets{}
	)

	secrets.IlpSecret, err = getIlpSecret(ctx, c, q)
	if err != nil {
		return secrets, err
	}

	secrets.PsqlSecret, err = getPsqlSecret(ctx, c, q)
	if err != nil {
		return secrets, err
	}

	return secrets, nil
}

func getIlpSecret(ctx context.Context, c client.Client, q *crdv1beta1.QuestDB) (*v1.Secret, error) {
	var (
		ilpSecret *v1.Secret
		err       error
	)

	secrets := &v1.SecretList{}
	err = c.List(ctx, secrets, client.InNamespace(q.Namespace))
	if err != nil {
		return nil, err
	}

	for idx, secret := range secrets.Items {
		if secret.Annotations[crdv1beta1.AnnotationQuestDBName] == q.Name {
			if secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType] == "ilp" {
				if ilpSecret != nil {
					return nil, fmt.Errorf("multiple ilp secrets found for questdb %s in namespace %s", q.Name, q.Namespace)
				}
				ilpSecret = &secrets.Items[idx]

			}
		}
	}

	// todo: Check all expected ilp secret keys

	return ilpSecret, nil
}

func getPsqlSecret(ctx context.Context, c client.Client, q *crdv1beta1.QuestDB) (*v1.Secret, error) {
	var (
		psqlSecret *v1.Secret
		err        error
	)

	secrets := &v1.SecretList{}

	err = c.List(ctx, secrets, client.InNamespace(q.Namespace))
	if err != nil {
		return nil, err
	}

	for idx, secret := range secrets.Items {
		if secret.Annotations[crdv1beta1.AnnotationQuestDBName] == q.Name {
			if secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType] == "psql" {
				if psqlSecret != nil {
					return nil, fmt.Errorf("multiple psql secrets found for questdb %s in namespace %s", q.Name, q.Namespace)
				}
				psqlSecret = &secrets.Items[idx]

			}
		}
	}

	// todo: Check all expected psql secret keys

	return psqlSecret, nil

}
