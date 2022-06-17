package handler

import (
	"alfred-bot/cmd/bot/commands/rotacommand/models/rotadetails"
	"alfred-bot/utils/db"
	"alfred-bot/utils/formatter"
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"time"
)

type RotaHandler struct {
	db *dynamodb.Client
}

func New(db *dynamodb.Client) *RotaHandler {
	return &RotaHandler{
		db: db,
	}
}

func (h *RotaHandler) GetRotaNames(channelId string) ([]string, error) {
	// TODO: Handle pagination
	out, err := h.db.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String(db.TableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: channelId},
		},
		ProjectionExpression: aws.String("sk"),
	})
	if err != nil {
		return nil, err
	}

	var rotaNames []string
	for _, v := range out.Items {
		var rotaDetails rotadetails.RotaDetails
		err = attributevalue.UnmarshalMap(v, &rotaDetails)
		if err != nil {
			return nil, err
		}

		rotaNames = append(rotaNames, rotaDetails.RotaName())
	}

	return rotaNames, nil
}

func (h *RotaHandler) GetRotaDetails(channelId string, rotaName string) (*rotadetails.RotaDetails, error) {
	out, err := h.db.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(db.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: channelId},
			"sk": &types.AttributeValueMemberS{Value: rotaName},
		},
	})
	if err != nil {
		return nil, err
	}

	var rotaDetails rotadetails.RotaDetails
	err = attributevalue.UnmarshalMap(out.Item, &rotaDetails)
	if err != nil {
		return nil, err
	}

	return &rotaDetails, nil
}

func (h *RotaHandler) GetEndingOnCallShifts() ([]*rotadetails.RotaDetails, error) {
	out, err := h.db.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName:        aws.String(db.TableName),
		FilterExpression: aws.String("attribute_exists(endOfShift) AND endOfShift <> :empty AND endOfShift <= :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":empty": &types.AttributeValueMemberS{Value: ""},
			":now":   &types.AttributeValueMemberS{Value: formatter.FormatTime(time.Now())},
		},
	})
	if err != nil {
		return nil, err
	}

	var rotas []*rotadetails.RotaDetails
	for _, v := range out.Items {
		var rotaDetails rotadetails.RotaDetails
		err = attributevalue.UnmarshalMap(v, &rotaDetails)
		if err != nil {
			return nil, err
		}

		rotas = append(rotas, &rotaDetails)
	}

	return rotas, nil
}

func (h *RotaHandler) SaveRotaDetails(channelId string, rotaName string, rotaMembers []string, rotaDuration string) error {
	var rotaMembersAsAttr []types.AttributeValue
	for _, v := range rotaMembers {
		rotaMembersAsAttr = append(rotaMembersAsAttr, &types.AttributeValueMemberS{Value: v})
	}

	_, err := h.db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(db.TableName),
		Item: map[string]types.AttributeValue{
			"pk":       &types.AttributeValueMemberS{Value: channelId},
			"sk":       &types.AttributeValueMemberS{Value: rotaName},
			"members":  &types.AttributeValueMemberL{Value: rotaMembersAsAttr},
			"duration": &types.AttributeValueMemberN{Value: rotaDuration},
		},
	})
	if err != nil {
		return err
	}

	return nil
}

func (h *RotaHandler) UpdateOnCallMember(channelId string, rotaName string, newOnCallMember string, startOfShift string, endOfShift string) error {
	_, err := h.db.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(db.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: channelId},
			"sk": &types.AttributeValueMemberS{Value: rotaName},
		},
		UpdateExpression: aws.String("set currOnCallMember = :currOnCallMember, startOfShift = :startOfShift, endOfShift = :endOfShift"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":currOnCallMember": &types.AttributeValueMemberS{Value: newOnCallMember},
			":startOfShift":     &types.AttributeValueMemberS{Value: startOfShift},
			":endOfShift":       &types.AttributeValueMemberS{Value: endOfShift},
		},
	})

	if err != nil {
		return err
	}

	return nil
}
