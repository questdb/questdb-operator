package secrets

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("internal/secrets", func() {

	Context("When getting QuestDB-related secrets", func() {
		var (
			ns string
		)

		BeforeEach(func() {
			ns = fmt.Sprintf("test-%d", time.Now().UnixNano())
			Expect(k8sClient.Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).To(Succeed())

			By("Ensuring that we start off with no secrets in the namespace")
			secretList := &v1.SecretList{}
			Expect(k8sClient.List(ctx, secretList, client.InNamespace(ns))).To(Succeed())
			Expect(secretList.Items).To(HaveLen(0))

		})

		It("Should set all secrets to nil if none are found", func() {
			secrets, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets.IlpSecret).To(BeNil())
			Expect(secrets.PsqlSecret).To(BeNil())
		})

		It("Should set all secrets to nil if a matching questdb is not found", func() {

			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "not-test",
						crdv1beta1.AnnotationQuestDBSecretType: "ilp",
					},
				},
			})).To(Succeed())

			secrets, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets.IlpSecret).To(BeNil())
			Expect(secrets.PsqlSecret).To(BeNil())
		})

		It("Should set an ilp secret if valid one is found", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "ilp",
					},
				},
				StringData: map[string]string{
					"auth.json": "{}",
				},
			})).To(Succeed())

			secrets, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets.IlpSecret).ToNot(BeNil())
			Expect(secrets.IlpSecret.Name).To(Equal("test"))
			Expect(secrets.PsqlSecret).To(BeNil())
		})

		It("Should set a psql secret if one is found", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "psql",
					},
				},
				StringData: map[string]string{
					"QDB_PG_USER":     "test",
					"QDB_PG_PASSWORD": "test",
				},
			})).To(Succeed())

			secrets, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(secrets.IlpSecret).To(BeNil())
			Expect(secrets.PsqlSecret).ToNot(BeNil())
			Expect(secrets.PsqlSecret.Name).To(Equal("test"))
		})

		It("Should return an error with an invalid secret type", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "invalid",
					},
				},
			})).To(Succeed())

			_, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error with 2 secrets of the same type", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "ilp",
					},
				},
			})).To(Succeed())

			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test2",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "ilp",
					},
				},
			})).To(Succeed())

			_, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error with an invalid ilp secret", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "ilp",
					},
				},
				StringData: map[string]string{
					"invalid": "{}",
				},
			})).To(Succeed())

			_, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).To(HaveOccurred())
		})

		It("should return an error with an invalid psql secret", func() {
			Expect(k8sClient.Create(ctx, &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: ns,
					Annotations: map[string]string{
						crdv1beta1.AnnotationQuestDBName:       "test",
						crdv1beta1.AnnotationQuestDBSecretType: "psql",
					},
				},
				StringData: map[string]string{
					"invalid": "{}",
				},
			})).To(Succeed())

			_, err := GetSecrets(ctx, k8sClient, client.ObjectKey{
				Name:      "test",
				Namespace: ns,
			})
			Expect(err).To(HaveOccurred())
		})
	})

})
