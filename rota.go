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
	Pk               string // ChannelID
	Sk               string // RotaName
	Members          []string
	CurrOnCallMember string
	Duration         string
	StartOfShift     string
	EndOfShift       string
}

type RotaCommand struct {
	db     *dynamodb.Client
	client *slack.Client
}

func NewRotaCommand(db *dynamodb.Client, client *slack.Client) *RotaCommand {
	return &RotaCommand{
		db:     db,
		client: client,
	}
}

const TableName = "rotas"
const rotaActions = "rota_actions"
const StartRotaAction = "start_rota"
const StopRotaAction = "stop_rota"
const UpdateRotaPromptAction = "update_rota_prompt"
const rotaNameAction = "set_rota_name"
const rotaMembersAction = "select_rota_members"
const rotaNameBlock = "rota_name"
const rotaMembersBlock = "rota_members"
const UpdateRotaCallback = "update_rota"

func (c *RotaCommand) StartRota(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	channelId := interaction.Channel.ID

	rotaDetails, err := c.getRotaDetails(channelId, "details")
	if err != nil {
		return err
	}

	userId := interaction.User.ID
	if len(rotaDetails.Members) == 0 {
		attachment := slack.Attachment{}
		attachment.Text = "Sorry, I can't start an empty rota!"
		attachment.Color = "#f0303a"

		err = c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	currOnCallMember := rotaDetails.CurrOnCallMember
	if currOnCallMember != "" {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("The rota has already begun. The current on-call person is: %s", atUserId(currOnCallMember))
		attachment.Color = "#f0303a"

		err = c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	newOnCallMember := action.Value
	if newOnCallMember != "" {
		err := c.updateOnCallMember(channelId, "details", newOnCallMember)
		if err != nil {
			return err
		}

		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(newOnCallMember))
		attachment.Color = "#4af030"
		_, _, err = c.client.PostMessage(channelId, slack.MsgOptionAttachments(attachment))
		if err != nil {
			return err
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

	return nil
}

func (c *RotaCommand) StopRota() error {
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

func (c *RotaCommand) Prompt(command slack.SlashCommand) (interface{}, error) {
	rotaNames, err := c.getRotaNames(command.ChannelID)
	if err != nil {
		return nil, err
	}

	if len(rotaNames) == 0 {
		// TODO
	}

	optionBlockObjects := make([]*slack.OptionBlockObject, 0, len(rotaNames))
	for _, v := range rotaNames {
		optionText := slack.NewTextBlockObject(slack.PlainTextType, v, false, false)
		optionBlockObjects = append(optionBlockObjects, slack.NewOptionBlockObject(v, optionText, nil))
	}

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.PlainTextType,
					Text: "Which rota do you want to look at?",
				},
				nil,
				slack.NewAccessory(
					slack.NewOptionsSelectBlockElement(
						slack.OptTypeStatic,
						nil,
						"rota_selection",
						optionBlockObjects...
					),
				),
			),
		},
	}

	return &attachment, nil
	//rotaDetails, err := c.getRotaDetails(command.ChannelID, "details")
	//if err != nil {
	//	return nil, err
	//}
	//
	//rotaMembers := rotaDetails.Members
	//currOnCallMember := rotaDetails.CurrOnCallMember
	//
	//return c.rotaDetailsPrompt(rotaMembers, currOnCallMember), nil
}

func (c *RotaCommand) UpdateRotaPrompt(channel slack.Channel, triggerId string, action *slack.BlockAction) error {
	rotaDetails, err := c.getRotaDetails(channel.ID, "details")
	if err != nil {
		return err
	}

	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Update your rota", false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false)

	rotaNameText := slack.NewTextBlockObject(slack.PlainTextType, "Rota Name", false, false)
	rotaNamePlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "New rota name", false, false)
	rotaNameElement := slack.NewPlainTextInputBlockElement(rotaNamePlaceholder, rotaNameAction)
	rotaNameElement.MaxLength = 50
	rotaNameElement.MinLength = 5
	rotaNameElement.InitialValue = rotaDetails.rotaName()
	rotaNameBlock := slack.NewInputBlock(rotaNameBlock, rotaNameText, rotaNameElement)

	rotaMemberSelectionText := slack.NewTextBlockObject(slack.PlainTextType, "Select members of your rota", false, false)
	rotaMemberSelectionElement := &slack.MultiSelectBlockElement{
		Type:         slack.MultiOptTypeUser,
		ActionID:     rotaMembersAction,
		Placeholder:  nil,
		InitialUsers: rotaDetails.Members,
	}
	rotaMemberSelectionBlock := slack.NewInputBlock(rotaMembersBlock, rotaMemberSelectionText, rotaMemberSelectionElement)

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			rotaNameBlock,
			rotaMemberSelectionBlock,
		},
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = "modal"
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.CallbackID = UpdateRotaCallback
	modalRequest.PrivateMetadata = channel.ID

	_, err = c.client.OpenView(triggerId, modalRequest)
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) UpdateRota(interaction *slack.InteractionCallback) error {
	view := interaction.View
	channelId := view.PrivateMetadata
	inputs := view.State.Values
	rotaName := inputs[rotaNameBlock][rotaNameAction].Value
	rotaMembers := inputs[rotaMembersBlock][rotaMembersAction].SelectedUsers

	err := c.saveRotaDetails(channelId, rotaName, rotaMembers)
	if err != nil {
		return err
	}

	prompt := c.rotaDetailsPrompt(rotaMembers, "")
	prompt.Color = "#4af030"

	err = c.respondToClient(channelId, interaction.User.ID, prompt)
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) rotaDetailsPrompt(rotaMembers []string, currOnCallMember string) *slack.Attachment {
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
			nil,
		),
	)

	rotaActionsBlock := slack.NewActionBlock(
		rotaActions,
		&slack.ButtonBlockElement{
			Type:     "button",
			ActionID: UpdateRotaPromptAction,
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

	return &attachment
}

func (c *RotaCommand) respondToClient(channelId string, userId string, payload *slack.Attachment) error {
	_, err := c.client.PostEphemeral(channelId, userId, slack.MsgOptionAttachments(*payload))
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) getRotaNames(channelId string) ([]string, error) {
	// TODO: Handle pagination
	out, err := c.db.Query(context.TODO(), &dynamodb.QueryInput{
		TableName:              aws.String(TableName),
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
		var rotaDetails RotaDetails
		err = attributevalue.UnmarshalMap(v, &rotaDetails)
		if err != nil {
			return nil, err
		}

		rotaNames = append(rotaNames, rotaDetails.rotaName())
	}

	return rotaNames, nil
}

func (c *RotaCommand) getRotaDetails(channelId string, rotaName string) (*RotaDetails, error) {
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

func (c *RotaCommand) saveRotaDetails(channelId string, rotaName string, rotaMembers []string) error {
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
func (c *RotaCommand) updateOnCallMember(channelId string, rotaName string, newOnCallMember string) error {
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

func (rd *RotaDetails) rotaName() string {
	return rd.Sk
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
