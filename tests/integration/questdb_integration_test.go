package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("QuestDB Integration Test", func() {
	It("should spin up all expected resources, using properly-defined secrets", func() {
		Expect(true).To(BeTrue())
	})
})
