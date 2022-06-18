package handler

import (
	"alfred-bot/config"
	"alfred-bot/utils/db"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestRotaDetails(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RotaDetails Suite")
}

var _ = BeforeSuite(func() {
	config.BootstrapEnv(true)
})

var _ = Describe("RotaHandler", func() {
	var dbHandler *db.Database
	var rotaHandler *RotaHandler

	BeforeEach(func() {
		dbHandler = db.New()
		rotaHandler = New(dbHandler)
	})

	AfterEach(func() {
		dbHandler.DeleteTable()
	})

	Describe("GetRotaNames", func() {
		Context("When there are no rotas", func() {
			It("Returns an empty response", func() {
				res, err := rotaHandler.GetRotaNames("dummyId")
				Expect(err).To(BeNil())
				Expect(len(res)).To(Equal(0))
			})
		})

		Context("When there are rotas avail", func() {
			BeforeEach(func() {
				_ = rotaHandler.SaveRotaDetails("dummyId", "dummyRota", []string{}, "1")
			})

			It("Returns a non-empty response", func() {
				res, err := rotaHandler.GetRotaNames("dummyId")
				Expect(err).To(BeNil())
				Expect(res).To(Equal([]string{"dummyRota"}))
			})
		})
	})

	Describe("GetRotaDetails", func() {
		Context("When rota does not exist", func() {
			It("Returns nil", func() {
				res, err := rotaHandler.GetRotaDetails("dummyId", "dummyRota")
				Expect(err).To(BeNil())
				Expect(res).To(BeNil())
			})
		})

		Context("When rota does exist", func() {
			BeforeEach(func() {
				_ = rotaHandler.SaveRotaDetails("dummyId", "dummyRota", []string{}, "1")
			})

			It("Returns the rota", func() {
				res, err := rotaHandler.GetRotaDetails("dummyId", "dummyRota")
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
			})
		})
	})
})
