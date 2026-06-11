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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	crdv1beta2 "github.com/questdb/questdb-operator/api/v1beta2"
)

func credsSecret(password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s"},
		Data: map[string][]byte{
			envPgUser:     []byte("admin"),
			envPgPassword: []byte(password),
		},
	}
}

func TestGeneratePassword(t *testing.T) {
	p, err := generatePassword()
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}
	if len(p) != 24 {
		t.Errorf("len = %d, want 24", len(p))
	}
	if p2, _ := generatePassword(); p == p2 {
		t.Errorf("expected two generated passwords to differ")
	}
}

func TestCredentialsSecretName(t *testing.T) {
	qdb := &crdv1beta2.QuestDB{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	if got := credentialsSecretName(qdb); got != "foo-credentials" {
		t.Errorf("generated name = %q, want foo-credentials", got)
	}
	if !usesGeneratedCredentials(qdb) {
		t.Errorf("expected usesGeneratedCredentials=true when auth.psql is nil")
	}

	qdb.Spec.Auth.Psql = &crdv1beta2.QuestDBPsqlAuthSpec{SecretName: "my-secret"}
	if got := credentialsSecretName(qdb); got != "my-secret" {
		t.Errorf("referenced name = %q, want my-secret", got)
	}
	if usesGeneratedCredentials(qdb) {
		t.Errorf("expected usesGeneratedCredentials=false when auth.psql.secretName is set")
	}
}

func TestValidateCredentialsSecret(t *testing.T) {
	if err := validateCredentialsSecret(credsSecret("pw")); err != nil {
		t.Errorf("valid secret rejected: %v", err)
	}
	missing := &corev1.Secret{Data: map[string][]byte{envPgUser: []byte("admin")}}
	if err := validateCredentialsSecret(missing); err == nil {
		t.Errorf("expected error for secret missing the password key")
	}
	if err := validateCredentialsSecret(credsSecret("")); err == nil {
		t.Errorf("expected error for empty password value")
	}
}

func TestHashCredentialsSecret(t *testing.T) {
	h1 := hashCredentialsSecret(credsSecret("pw"))
	h2 := hashCredentialsSecret(credsSecret("pw"))
	if h1 != h2 {
		t.Errorf("hash should be stable for identical credentials")
	}
	if h1 == hashCredentialsSecret(credsSecret("pw2")) {
		t.Errorf("hash should change when the password changes")
	}
}
