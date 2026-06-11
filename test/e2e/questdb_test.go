//go:build e2e

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

package e2e

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/questdb/questdb-operator/test/utils"
)

// questdbNamespace is where the QuestDB custom resources are created during e2e.
const questdbNamespace = "questdb-e2e"

// kubectlApply applies a manifest passed on stdin.
func kubectlApply(manifest string) {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed to apply manifest")
}

// This suite exercises the operator end-to-end against a real (Kind) cluster: it deploys the
// controller, brings up a QuestDB, and — when the cluster has a snapshot-capable CSI — runs a
// real checkpoint-bracketed VolumeSnapshot backup. The backup spec Skips itself when no
// VolumeSnapshot CRD is present, so the lifecycle test runs on a vanilla Kind cluster and the
// backup test only runs where snapshot-controller + a CSI like csi-driver-hostpath are installed.
var _ = Describe("QuestDB", Ordered, func() {
	BeforeAll(func() {
		By("deploying the controller-manager")
		_, err := utils.Run(exec.Command("make", "deploy", "IMG="+managerImage))
		Expect(err).NotTo(HaveOccurred(), "failed to deploy the controller-manager")

		By("creating the QuestDB test namespace")
		_, _ = utils.Run(exec.Command("kubectl", "create", "ns", questdbNamespace))
	})

	AfterAll(func() {
		By("removing QuestDB resources and the test namespace")
		_, _ = utils.Run(exec.Command("kubectl", "delete", "questdbbackup,questdb", "--all", "-n", questdbNamespace, "--ignore-not-found"))
		_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", questdbNamespace, "--ignore-not-found"))

		By("undeploying the controller-manager")
		_, _ = utils.Run(exec.Command("make", "undeploy"))
	})

	It("brings up a single-pod StatefulSet and generates credentials", func() {
		By("applying a QuestDB")
		kubectlApply(`apiVersion: crd.questdb.io/v1beta2
kind: QuestDB
metadata:
  name: e2e
  namespace: ` + questdbNamespace + `
spec:
  volume:
    size: 1Gi
`)

		By("waiting for the StatefulSet to report a ready replica (its readiness probe is the :9003 health server)")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "statefulset", "e2e",
				"-n", questdbNamespace, "-o", "jsonpath={.status.readyReplicas}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("1"))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("verifying the operator generated a pg-wire credentials Secret")
		_, err := utils.Run(exec.Command("kubectl", "get", "secret", "e2e-credentials", "-n", questdbNamespace))
		Expect(err).NotTo(HaveOccurred(), "expected generated credentials Secret e2e-credentials")

		By("verifying the QuestDB reports Ready")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "questdb", "e2e", "-n", questdbNamespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("True"))
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("completes a checkpoint-bracketed VolumeSnapshot backup", func() {
		By("checking the cluster has a VolumeSnapshot CRD")
		if _, err := utils.Run(exec.Command("kubectl", "get", "crd", "volumesnapshots.snapshot.storage.k8s.io")); err != nil {
			Skip("no VolumeSnapshot CRD; install snapshot-controller + a snapshot-capable CSI (e.g. csi-driver-hostpath) to run the backup e2e")
		}

		By("creating a QuestDBBackup")
		kubectlApply(`apiVersion: crd.questdb.io/v1beta2
kind: QuestDBBackup
metadata:
  name: e2e-backup
  namespace: ` + questdbNamespace + `
spec:
  questdbName: e2e
`)

		By("waiting for the backup to reach Succeeded (CHECKPOINT CREATE -> VolumeSnapshot ready -> CHECKPOINT RELEASE)")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "questdbbackup", "e2e-backup", "-n", questdbNamespace,
				"-o", "jsonpath={.status.phase}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("Succeeded"))
		}, 5*time.Minute, 10*time.Second).Should(Succeed())

		By("verifying the VolumeSnapshot was created and is ready")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "volumesnapshot", "e2e-backup", "-n", questdbNamespace,
				"-o", "jsonpath={.status.readyToUse}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(Equal("true"))
		}, time.Minute, 5*time.Second).Should(Succeed())
	})
})
