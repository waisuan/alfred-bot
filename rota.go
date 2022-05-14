package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/slack-go/slack"
	"strings"
)

type RotaDetails struct {
	ChannelId        string   `json:"pk"`
	RotaName         string   `json:"sk"`
	Members          []string `json:"members"`
	CurrOnCallMember string   `json:"currOnCallMember"`
	Duration         string   `json:"duration"`
	StartOfShift     string   `json:"startOfShift"`
	EndOfShift       string   `json:"endOfShift"`
}

type Command struct {
	db *dynamodb.Client
}

func NewCommand(db *dynamodb.Client) *Command {
	return &Command{
		db: db,
	}
}

const TableName = "rotas"
const rotaActions = "rota_actions"
const SelectRotaMembersAction = "select_rota_members"
const StartRotaAction = "start_rota"
const StopRotaAction = "stop_rota"
const UpdateRotaAction = "update_rota"

func (c *Command) StartRota(channel slack.Channel, action *slack.BlockAction, client *slack.Client) (*slack.Attachment, error) {
	rotaDetails, err := c.getRotaDetails(channel.ID, "details")
	if err != nil {
		return nil, err
	}

	if len(rotaDetails.Members) == 0 {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("Sorry, I can't start an empty rota!")
		attachment.Color = "#f0303a"

		return &attachment, nil
	}

	currOnCallMember := rotaDetails.CurrOnCallMember
	if currOnCallMember != "" {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("The rota has already begun. The current on-call person is: %s", atUserId(currOnCallMember))
		attachment.Color = "#f0303a"

		return &attachment, nil
	}

	newOnCallMember := action.Value
	if newOnCallMember != "" {
		err := c.updateOnCallMember(channel.ID, "details", newOnCallMember)
		if err != nil {
			return nil, err
		}

		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(newOnCallMember))
		attachment.Color = "#4af030"
		_, _, err = client.PostMessage(channel.ID, slack.MsgOptionAttachments(attachment))
		if err != nil {
			return nil, err
		}
	}

	//go func() {
	//	newOnCallMember := action.Value
	//	for {
	//		if newOnCallMember != "" {
	//			attachment := slack.Attachment{}
	//			attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(c.currOnCallMember))
	//			attachment.Color = "#4af030"
	//			_, _, err := client.PostMessage(c.channelId, slack.MsgOptionAttachments(attachment))
	//			if err != nil {
	//				log.Println(err)
	//			}
	//		}
	//
	//		time.Sleep(30 * time.Second)
	//
	//		for idx, member := range c.rotaList {
	//			if c.currOnCallMember == member {
	//				c.currOnCallMember = c.rotaList[(idx+1)%len(c.rotaList)]
	//				break
	//			}
	//		}
	//	}
	//}()

	return nil, nil
}

func (c *Command) StopRota(client *slack.Client) error {
	//c.currOnCallMember = ""
	//
	//attachment := slack.Attachment{}
	//attachment.Text = fmt.Sprintf("OK, I've stopped your rota!")
	//attachment.Color = "#4af030"
	//_, _, err := client.PostMessage(c.channelId, slack.MsgOptionAttachments(attachment))
	//if err != nil {
	//	return err
	//}

	return nil
}

func (c *Command) HandlePrompt(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	rotaDetails, err := c.getRotaDetails(command.ChannelID, "details")
	if err != nil {
		return nil, err
	}

	rotaMembers := rotaDetails.Members
	currOnCallMember := rotaDetails.CurrOnCallMember

	var currRotaMembersText string
	var currOnCallMemberText string
	if len(rotaMembers) > 0 {
		currRotaMembersText = fmt.Sprintf("Current rota members:\n%s", rotaMembersAsString(rotaMembers))

		if currOnCallMember != "" {
			currOnCallMemberText = fmt.Sprintf("*The current on-call person is: %s*", atUserId(currOnCallMember))
		} else {
			currOnCallMemberText = "*The rota has not started yet.*"
		}
	} else {
		currRotaMembersText = "The rota is empty. Shall we pull in some members?"
	}

	var blocks []slack.Block
	if currOnCallMemberText != "" {
		blocks = append(blocks,
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: currOnCallMemberText,
				},
				nil,
				nil,
			),
		)
	}

	blocks = append(blocks,
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: currRotaMembersText,
			},
			nil,
			slack.NewAccessory(
				&slack.MultiSelectBlockElement{
					Type:         slack.MultiOptTypeUser,
					ActionID:     SelectRotaMembersAction,
					Placeholder:  &slack.TextBlockObject{Text: "Select members of your rota", Type: slack.PlainTextType},
					InitialUsers: rotaMembers,
				},
			),
		),
	)

	rotaActionsBlock := slack.NewActionBlock(
		rotaActions,
		&slack.ButtonBlockElement{
			Type:     "button",
			ActionID: UpdateRotaAction,
			Text:     &slack.TextBlockObject{Text: "Update the rota", Type: slack.PlainTextType},
			Style:    slack.StyleDefault,
		},
	)

	if len(rotaMembers) > 0 {
		if currOnCallMember == "" {
			rotaActionsBlock.Elements.ElementSet = append(
				rotaActionsBlock.Elements.ElementSet,
				&slack.ButtonBlockElement{
					Type:     "button",
					ActionID: StartRotaAction,
					Text:     &slack.TextBlockObject{Text: "Start the rota", Type: slack.PlainTextType},
					Style:    slack.StylePrimary,
					Value:    rotaMembers[0],
				},
			)
		} else {
			rotaActionsBlock.Elements.ElementSet = append(
				rotaActionsBlock.Elements.ElementSet,
				&slack.ButtonBlockElement{
					Type:     "button",
					ActionID: StopRotaAction,
					Text:     &slack.TextBlockObject{Text: "Stop the rota", Type: slack.PlainTextType},
					Style:    slack.StyleDanger,
				},
			)
		}
	}

	blocks = append(blocks, rotaActionsBlock)

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{BlockSet: blocks}

	return attachment, nil
}

func (c *Command) HandleSelection(channel slack.Channel, action *slack.BlockAction, client *slack.Client) (*slack.Attachment, error) {
	var newRotaMembers []string
	for _, userId := range action.SelectedUsers {
		user, err := client.GetUserInfo(userId)
		if err != nil {
			return nil, err
		}
		newRotaMembers = append(newRotaMembers, user.ID)
	}

	var postSelectionText string
	if len(newRotaMembers) > 0 {
		postSelectionText = fmt.Sprintf("The rota now consists of:\n%s", rotaMembersAsString(newRotaMembers))
	} else {
		postSelectionText = "There are no members set to the rota."
	}

	err := c.saveRotaDetails(channel.ID, "details", newRotaMembers)
	if err != nil {
		return nil, err
	}

	var blocks []slack.Block
	blocks = append(blocks,
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: postSelectionText,
			},
			nil,
			nil,
		),
	)

	if len(newRotaMembers) > 0 {
		blocks = append(blocks,
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: fmt.Sprintf("The new on-call person will be %s", atUserId(newRotaMembers[0])),
				},
				nil,
				slack.NewAccessory(
					&slack.ButtonBlockElement{
						Type:     "button",
						ActionID: StartRotaAction,
						Text:     &slack.TextBlockObject{Text: "Start the rota", Type: slack.PlainTextType},
						Style:    slack.StylePrimary,
						Value:    newRotaMembers[0],
					},
				),
			),
		)
	}

	attachment := slack.Attachment{}
	attachment.Color = "#4af030"
	attachment.Blocks = slack.Blocks{BlockSet: blocks}

	return &attachment, nil
}

func (c *Command) getRotaDetails(channelId string, rotaName string) (*RotaDetails, error) {
	out, err := c.db.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: channelId},
			"sk": &types.AttributeValueMemberS{Value: rotaName},
		},
	})
	if err != nil {
		return nil, err
	}

	var rotaDetails RotaDetails
	err = attributevalue.UnmarshalMap(out.Item, &rotaDetails)
	if err != nil {
		return nil, err
	}

	return &rotaDetails, nil
}

func (c *Command) saveRotaDetails(channelId string, rotaName string, rotaMembers []string) error {
	var rotaMembersAsAttr []types.AttributeValue
	for _, v := range rotaMembers {
		rotaMembersAsAttr = append(rotaMembersAsAttr, &types.AttributeValueMemberS{Value: v})
	}

	_, err := c.db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(TableName),
		Item: map[string]types.AttributeValue{
			"pk":      &types.AttributeValueMemberS{Value: channelId},
			"sk":      &types.AttributeValueMemberS{Value: rotaName},
			"members": &types.AttributeValueMemberL{Value: rotaMembersAsAttr},
		},
	})
	if err != nil {
		return err
	}

	return nil
}
func (c *Command) updateOnCallMember(channelId string, rotaName string, newOnCallMember string) error {
	_, err := c.db.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: channelId},
			"sk": &types.AttributeValueMemberS{Value: rotaName},
		},
		UpdateExpression: aws.String("set currOnCallMember = :currOnCallMember"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":currOnCallMember": &types.AttributeValueMemberS{Value: newOnCallMember},
		},
	})

	if err != nil {
		return err
	}

	return nil
}

func rotaMembersAsString(members []string) string {
	var formattedUserIds []string
	for _, userId := range members {
		formattedUserIds = append(formattedUserIds, fmt.Sprintf("• %s", atUserId(userId)))
	}
	return fmt.Sprintf("%s", strings.Join(formattedUserIds, "\n"))
}

func atUserId(userId string) string {
	return fmt.Sprintf("<@%s>", userId)
}
