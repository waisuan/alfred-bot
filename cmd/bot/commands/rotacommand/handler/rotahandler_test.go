package handler

import (
	"alfred-bot/utils/db"
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestRotaDetails(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RotaDetails Suite")
}

const tableName = "rotas_test"

var _ = Describe("GetRotaNames", func() {
	var dbClient *dynamodb.Client
	var rotaHandler *RotaHandler

	BeforeEach(func() {
		dbClient = db.Init(tableName)
		rotaHandler = New(dbClient)
	})

	AfterEach(func() {
		_, _ = dbClient.DeleteTable(context.TODO(), &dynamodb.DeleteTableInput{TableName: aws.String(tableName)})
	})

	Context("When there are no rotas", func() {
		It("Returns an empty response", func() {
			res, err := rotaHandler.GetRotaNames("dummy")
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
