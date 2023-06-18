package secrets

import (
	"context"
	"fmt"

	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QuestDBSecrets is a container for v1.Secrets that may be used
// as credentials for a QuestDB instance.
type QuestDBSecrets struct {
	IlpSecret  *v1.Secret
	PsqlSecret *v1.Secret
}

// GetSecrets uses annotations to find secrets that are related to the QuestDB defined by the
// provided NamespacedName. Currently, 2 types of secrets are supported: a JWK for securing ILP
// connections, and a username/password combination for the pgwire connection.
// If a secret is not found in the same namespace as the QuestDB, it will be nil.
//
// Secrets need to have 2 annotations in order to be retrieved by this function:
// questdb.crd.questdb.io/name=<your-questdb-name>
// questdb.crd.questdb.io/secret-type=<ilp/psql>
func GetSecrets(ctx context.Context, c client.Client, qdb types.NamespacedName) (QuestDBSecrets, error) {
	var (
		err        error
		secrets    = QuestDBSecrets{}
		secretList = &v1.SecretList{}
	)

	err = c.List(ctx, secretList, client.InNamespace(qdb.Namespace))
	if err != nil {
		return secrets, err
	}

	for idx, secret := range secretList.Items {
		if secret.Annotations[crdv1beta1.AnnotationQuestDBName] == qdb.Name {
			switch secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType] {
			case "ilp":
				if secrets.IlpSecret != nil {
					return secrets, fmt.Errorf("multiple ilp secrets found for questdb %s in namespace %s", qdb.Name, qdb.Namespace)
				}
				secrets.IlpSecret = &secretList.Items[idx]
			case "psql":
				if secrets.PsqlSecret != nil {
					return secrets, fmt.Errorf("multiple psql secrets found for questdb %s in namespace %s", qdb.Name, qdb.Namespace)
				}
				secrets.PsqlSecret = &secretList.Items[idx]
			default:
				return secrets, fmt.Errorf("invalid secret type %s for questdb %s in namespace %s", secret.Annotations[crdv1beta1.AnnotationQuestDBSecretType], qdb.Name, qdb.Namespace)
			}
		}
	}

	if err := validateIlpSecret(secrets.IlpSecret, qdb); err != nil {
		return secrets, err
	}

	if err := validatePsqlSecret(secrets.PsqlSecret, qdb); err != nil {
		return secrets, err
	}

	return secrets, nil
}

func validateIlpSecret(secret *v1.Secret, n types.NamespacedName) error {
	if secret == nil {
		return nil
	}

	if val, found := secret.Data["auth.json"]; !found {
		return newInvalidSecretErrorf("ilp secret for questdb %s in namespace %s is missing auth.json", n.Name, n.Namespace)
	} else if string(val) == "" {
		return newInvalidSecretErrorf("ilp secret for questdb %s in namespace %s has empty auth.json", n.Name, n.Namespace)
	}

	return nil
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
			return newInvalidSecretErrorf("psql secret for questdb %s in namespace %s is missing required keys", n.Name, n.Namespace)
		}
	}

	return nil

}
