/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

// credentialsSecretName returns the name of the Secret holding pg-wire credentials for a QuestDB:
// the referenced Secret when spec.auth.psql is set, otherwise the operator-generated "<name>-credentials".
func credentialsSecretName(qdb *crdv1beta2.QuestDB) string {
	if qdb.Spec.Auth.Psql != nil && qdb.Spec.Auth.Psql.SecretName != "" {
		return qdb.Spec.Auth.Psql.SecretName
	}
	return qdb.Name + "-credentials"
}

// usesGeneratedCredentials reports whether the operator owns/generates the credentials Secret.
func usesGeneratedCredentials(qdb *crdv1beta2.QuestDB) bool {
	return qdb.Spec.Auth.Psql == nil || qdb.Spec.Auth.Psql.SecretName == ""
}

// reconcileCredentials ensures a pg-wire credentials Secret exists and returns it. When the user
// references a Secret it is fetched (and may be missing/invalid — the caller surfaces that as a
// condition). When no Secret is referenced, "<name>-credentials" is generated with a random
// password the first time and reused thereafter.
func (r *QuestDBReconciler) reconcileCredentials(ctx context.Context, qdb *crdv1beta2.QuestDB) (*corev1.Secret, error) {
	name := credentialsSecretName(qdb)
	key := types.NamespacedName{Namespace: qdb.Namespace, Name: name}

	secret := &corev1.Secret{}
	getErr := r.Get(ctx, key, secret)

	if !usesGeneratedCredentials(qdb) {
		// User-referenced Secret: never created or mutated by the operator.
		return secret, getErr
	}

	if getErr == nil {
		return secret, nil
	}
	if !apierrors.IsNotFound(getErr) {
		return nil, getErr
	}

	// Generate a new credentials Secret owned by the QuestDB.
	password, err := generatePassword(24)
	if err != nil {
		return nil, err
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: qdb.Namespace},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			envPgUser:     []byte(defaultPgUser),
			envPgPassword: []byte(password),
		},
	}
	if err := controllerutil.SetControllerReference(qdb, secret, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, secret); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// Lost a race with another reconcile; adopt the existing Secret.
			if getErr := r.Get(ctx, key, secret); getErr != nil {
				return nil, getErr
			}
			return secret, nil
		}
		return nil, err
	}
	return secret, nil
}

// validateCredentialsSecret checks that a credentials Secret carries both required keys, non-empty.
func validateCredentialsSecret(secret *corev1.Secret) error {
	for _, k := range []string{envPgUser, envPgPassword} {
		if len(secret.Data[k]) == 0 {
			return fmt.Errorf("credentials secret %q is missing or has an empty %q key", secret.Name, k)
		}
	}
	return nil
}

// hashCredentialsSecret returns a stable hash of the pg-wire credentials, used as a pod-template
// annotation so rotating the Secret rolls the StatefulSet.
func hashCredentialsSecret(secret *corev1.Secret) string {
	h := sha256.New()
	h.Write(secret.Data[envPgUser])
	h.Write([]byte{0})
	h.Write(secret.Data[envPgPassword])
	return hex.EncodeToString(h.Sum(nil))
}

// generatePassword returns a random alphanumeric password of length n (ambiguous characters omitted).
func generatePassword(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf), nil
}
