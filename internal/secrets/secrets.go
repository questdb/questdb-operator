package secrets

import (
	"context"
	"fmt"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type QuestDBSecrets struct {
	IlpSecret  *v1.Secret
	PsqlSecret *v1.Secret
}

func GetSecrets(ctx context.Context, c client.Client, n types.NamespacedName) (QuestDBSecrets, error) {
	var (
		err     error
		secrets = QuestDBSecrets{}
	)

	secrets.IlpSecret, err = getIlpSecret(ctx, c, n)
	if err != nil {
		return secrets, err
	}

	secrets.PsqlSecret, err = getPsqlSecret(ctx, c, n)
	if err != nil {
		return secrets, err
	}

	return secrets, nil
}

func getIlpSecret(ctx context.Context, c client.Client, n types.NamespacedName) (*v1.Secret, error) {
	var (
		ilpSecret *v1.Secret
		err       error
	)

	secrets := &v1.SecretList{}
	err = c.List(ctx, secrets, client.InNamespace(n.Namespace))
	if err != nil {
		return nil, err
	}

	for idx, secret := range secrets.Items {
		if secret.Annotations[crdv1beta1.AnnotationQuestDBName] == n.Name {
			if secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType] == "ilp" {
				if ilpSecret != nil {
					return nil, fmt.Errorf("multiple ilp secrets found for questdb %s in namespace %s", n.Name, n.Namespace)
				}
				ilpSecret = &secrets.Items[idx]

			}
		}
	}

	return ilpSecret, validateIlpSecret(ilpSecret, n)
}

func validateIlpSecret(secret *v1.Secret, n types.NamespacedName) error {
	if secret == nil {
		return nil
	}

	if val, found := secret.Data["auth.json"]; !found {
		return fmt.Errorf("ilp secret for questdb %s in namespace %s is missing auth.json", n.Name, n.Namespace)
	} else if string(val) == "" {
		return fmt.Errorf("ilp secret for questdb %s in namespace %s has empty auth.json", n.Name, n.Namespace)
	}

	return nil
}

func getPsqlSecret(ctx context.Context, c client.Client, n types.NamespacedName) (*v1.Secret, error) {
	var (
		psqlSecret *v1.Secret
		err        error
	)

	secrets := &v1.SecretList{}

	err = c.List(ctx, secrets, client.InNamespace(n.Namespace))
	if err != nil {
		return nil, err
	}

	for idx, secret := range secrets.Items {
		if secret.Annotations[crdv1beta1.AnnotationQuestDBName] == n.Name {
			if secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType] == "psql" {
				if psqlSecret != nil {
					return nil, fmt.Errorf("multiple psql secrets found for questdb %s in namespace %s", n.Name, n.Namespace)
				}
				psqlSecret = &secrets.Items[idx]

			}
		}
	}

	return psqlSecret, validatePsqlSecret(psqlSecret, n)

}

func validatePsqlSecret(s *v1.Secret, n types.NamespacedName) error {

	if s == nil {
		return nil
	}

	// Check for expected keys
	expectedKeys := map[string]bool{
		"QDB_PG_USER":     false,
		"QDB_PG_PASSWORD": false,
	}

	for k := range expectedKeys {
		v, found := s.Data[k]
		if !found {
			continue
		}

		if string(v) == "" {
			continue
		}

		expectedKeys[k] = true
	}

	for _, v := range expectedKeys {
		if !v {
			return fmt.Errorf("psql secret for questdb %s in namespace %s is missing required keys", n.Name, n.Namespace)
		}
	}

	return nil

}
