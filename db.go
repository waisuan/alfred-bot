package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const TableName = "rotas"

func InitDatabase() *dynamodb.Client {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	svc := dynamodb.NewFromConfig(cfg, func(options *dynamodb.Options) {
		options.Region = "eu-central-1"
		options.Credentials = credentials.StaticCredentialsProvider{
			Value: aws.Credentials{AccessKeyID: "dummy", SecretAccessKey: "dummy"},
		}
		options.EndpointResolver = dynamodb.EndpointResolverFromURL("http://localhost:8000")
	})

	_, err = svc.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{TableName: aws.String(TableName)})
	if err != nil {
		_, err := svc.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
			AttributeDefinitions: []types.AttributeDefinition{
				{
					AttributeName: aws.String("pk"),
					AttributeType: types.ScalarAttributeTypeS,
				},
				{
					AttributeName: aws.String("sk"),
					AttributeType: types.ScalarAttributeTypeS,
				},
			},
			KeySchema: []types.KeySchemaElement{
				{
					AttributeName: aws.String("pk"),
					KeyType:       types.KeyTypeHash,
				},
				{
					AttributeName: aws.String("sk"),
					KeyType:       types.KeyTypeRange,
				},
			},
			TableName:   aws.String(TableName),
			BillingMode: types.BillingModePayPerRequest,
		})
		if err != nil {
			panic(err)
		}
	}

	return svc
}
