package rotadetails

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

func TestRotaDetails(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RotaDetails Suite")
}

var _ = Describe("GenerateEndOfShift", func() {
	It("Runs", func() {
		Expect(GenerateEndOfShift(time.Now(), 1)).ToNot(BeNil())
	})
})
