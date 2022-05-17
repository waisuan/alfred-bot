package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/slack-go/slack"
	"strconv"
	"strings"
	"time"
)

type RotaDetails struct {
	Pk               string // ChannelID
	Sk               string // RotaName
	Members          []string
	CurrOnCallMember string
	Duration         int
	StartOfShift     string
	EndOfShift       string
}

type RotaCommandMetadata struct {
	ChannelId    string
	RotaName     string
	StartOfShift string
	EndOfShift   string
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
const StartRotaAction = "start_rota_prompt"
const StopRotaAction = "stop_rota"
const SelectRotaAction = "select_rota"
const UpdateRotaPromptAction = "update_rota_prompt"
const CreateRotaPromptAction = "create_rota_prompt"
const rotaActions = "rota_actions"
const promptActions = "prompt_actions"
const rotaNameAction = "set_rota_name"
const rotaMembersAction = "select_rota_members"
const rotaDurationAction = "set_rota_duration"
const rotaOnCallMemberAction = "set_on_call_member"
const rotaNameBlock = "rota_name"
const rotaMembersBlock = "rota_members"
const UpdateRotaCallback = "update_rota"
const CreateRotaCallback = "create_rota"
const StartRotaCallback = "start_rota"
const rotaDurationBlock = "rota_duration"
const rotaOnCallMemberBlock = "on_call_member"

func (c *RotaCommand) StartRotaPrompt(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	channelId := interaction.Channel.ID
	rotaName := action.Value

	rotaDetails, err := c.getRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	var unableToStartRotaErr string

	rotaMembers := rotaDetails.Members
	if len(rotaMembers) == 0 {
		unableToStartRotaErr = "Sorry, I can't start an empty rota!"
	}

	if rotaDetails.Duration == 0 {
		unableToStartRotaErr = "Sorry, I can't start a rota without an on-call shift duration!"
	}

	if rotaDetails.CurrOnCallMember != "" {
		unableToStartRotaErr = fmt.Sprintf("The rota has already begun. The current on-call person is: %s", atUserId(rotaDetails.CurrOnCallMember))
	}

	if unableToStartRotaErr != "" {
		userId := interaction.User.ID

		attachment := slack.Attachment{}
		attachment.Text = unableToStartRotaErr
		attachment.Color = "#f0303a"

		err = c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Start your rota", false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false)

	onCallMemberText := slack.NewTextBlockObject(slack.PlainTextType, "Who should be on-call for this shift?", false, false)
	onCallOptionBlockObjects := make([]*slack.OptionBlockObject, 0, len(rotaMembers))
	for _, v := range rotaMembers {
		optionText := slack.NewTextBlockObject(slack.PlainTextType, atUserId(v), false, false)
		onCallOptionBlockObjects = append(onCallOptionBlockObjects, slack.NewOptionBlockObject(v, optionText, nil))
	}
	onCallMemberElement := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, nil, rotaOnCallMemberAction, onCallOptionBlockObjects...)
	onCallMemberInputBlock := slack.NewInputBlock(rotaOnCallMemberBlock, onCallMemberText, onCallMemberElement)

	startOfShiftTime := time.Now()
	endOfShiftTime := rotaDetails.generateEndOfShift(startOfShiftTime)
	shiftDetailsBlock := slack.NewSectionBlock(
		&slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: fmt.Sprintf("*Their on-call shift will end on: %v*", endOfShiftTime),
		},
		nil,
		nil,
	)

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			onCallMemberInputBlock,
			shiftDetailsBlock,
		},
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = "modal"
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.CallbackID = StartRotaCallback

	metadata := RotaCommandMetadata{
		ChannelId:    channelId,
		RotaName:     rotaName,
		StartOfShift: formatTime(startOfShiftTime),
		EndOfShift:   endOfShiftTime,
	}
	b, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	modalRequest.PrivateMetadata = string(b)

	_, err = c.client.OpenView(interaction.TriggerID, modalRequest)
	if err != nil {
		return err
	}

	return nil

	//newOnCallMember := action.Value
	//if newOnCallMember != "" {
	//	err := c.updateOnCallMember(channelId, "details", newOnCallMember)
	//	if err != nil {
	//		return err
	//	}
	//
	//	attachment := slack.Attachment{}
	//	attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(newOnCallMember))
	//	attachment.Color = "#4af030"
	//	_, _, err = c.client.PostMessage(channelId, slack.MsgOptionAttachments(attachment))
	//	if err != nil {
	//		return err
	//	}
	//}

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

	var mainPromptBlock *slack.SectionBlock
	if len(rotaNames) == 0 {
		mainPromptBlock = slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.PlainTextType,
				Text: "Looks like this channel does not have any rotas. Shall we create one?",
			},
			nil,
			nil,
		)
	} else {
		rotaNameOptionBlockObjects := make([]*slack.OptionBlockObject, 0, len(rotaNames))
		for _, v := range rotaNames {
			optionText := slack.NewTextBlockObject(slack.PlainTextType, v, false, false)
			rotaNameOptionBlockObjects = append(rotaNameOptionBlockObjects, slack.NewOptionBlockObject(v, optionText, nil))
		}

		mainPromptBlock = slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.PlainTextType,
				Text: "Which rota do you want to look at?",
			},
			nil,
			slack.NewAccessory(
				slack.NewOptionsSelectBlockElement(
					slack.OptTypeStatic,
					nil,
					SelectRotaAction,
					rotaNameOptionBlockObjects...,
				),
			),
		)
	}

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			mainPromptBlock,
			slack.NewActionBlock(
				promptActions,
				&slack.ButtonBlockElement{
					Type:     "button",
					ActionID: CreateRotaPromptAction,
					Text:     &slack.TextBlockObject{Text: "Create a new rota", Type: slack.PlainTextType},
					Style:    slack.StyleDefault,
				},
			),
		},
	}

	return &attachment, nil
}

func (c *RotaCommand) PromptRotaDetails(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	rotaName := action.SelectedOption.Value
	channelId := interaction.Channel.ID

	rotaDetails, err := c.getRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	rotaMembers := rotaDetails.Members
	currOnCallMember := rotaDetails.CurrOnCallMember
	rotaDuration := rotaDetails.Duration
	endOfShift := rotaDetails.EndOfShift

	prompt := c.rotaDetailsPrompt(rotaMembers, currOnCallMember, rotaName, rotaDuration, endOfShift)
	err = c.respondToClient(channelId, interaction.User.ID, prompt)
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) CreateRotaPrompt(interaction *slack.InteractionCallback) error {
	channelId := interaction.Channel.ID
	triggerId := interaction.TriggerID

	return c.upsertRotaPrompt(channelId, triggerId, "", CreateRotaCallback)
}

func (c *RotaCommand) UpdateRotaPrompt(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	channelId := interaction.Channel.ID
	triggerId := interaction.TriggerID
	rotaName := action.Value

	return c.upsertRotaPrompt(channelId, triggerId, rotaName, UpdateRotaCallback)
}

func (c *RotaCommand) CreateRota(interaction *slack.InteractionCallback) error {
	view := interaction.View
	channelId := view.ExternalID
	inputs := view.State.Values
	rotaName := inputs[rotaNameBlock][rotaNameAction].Value
	rotaMembers := inputs[rotaMembersBlock][rotaMembersAction].SelectedUsers
	rotaDuration := inputs[rotaDurationBlock][rotaDurationAction].SelectedOption.Value

	return c.upsertRotaCallback(channelId, interaction.User.ID, rotaName, rotaMembers, rotaDuration)
}

func (c *RotaCommand) UpdateRota(interaction *slack.InteractionCallback) error {
	view := interaction.View
	channelId := view.ExternalID
	rotaName := view.PrivateMetadata
	inputs := view.State.Values
	rotaMembers := inputs[rotaMembersBlock][rotaMembersAction].SelectedUsers
	rotaDuration := inputs[rotaDurationBlock][rotaDurationAction].SelectedOption.Value

	return c.upsertRotaCallback(channelId, interaction.User.ID, rotaName, rotaMembers, rotaDuration)
}

func (c *RotaCommand) StartRota(interaction *slack.InteractionCallback) error {
	view := interaction.View
	inputs := view.State.Values
	onCallMember := inputs[rotaOnCallMemberBlock][rotaOnCallMemberAction].SelectedOption.Value

	var metadata RotaCommandMetadata
	err := json.Unmarshal([]byte(view.PrivateMetadata), &metadata)
	if err != nil {
		return err
	}

	err = c.updateOnCallMember(metadata.ChannelId, metadata.RotaName, onCallMember, metadata.StartOfShift, metadata.EndOfShift)
	if err != nil {
		return err
	}

	attachment := slack.Attachment{}
	attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(onCallMember))
	attachment.Color = "#4af030"
	_, _, err = c.client.PostMessage(metadata.ChannelId, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) rotaDetailsPrompt(rotaMembers []string, currOnCallMember string, rotaName string, rotaDuration int, endOfShift string) *slack.Attachment {
	var currRotaMembersText string
	var currOnCallMemberText string
	if len(rotaMembers) > 0 {
		currRotaMembersText = fmt.Sprintf("Current rota members:\n%s", rotaMembersAsString(rotaMembers))

		if currOnCallMember != "" {
			currOnCallMemberText = fmt.Sprintf("*The current on-call person is: %s (shift ends at %v)*", atUserId(currOnCallMember), endOfShift)
		} else {
			currOnCallMemberText = "*The rota has not started yet.*"
		}
	} else {
		currRotaMembersText = "The rota is empty. Shall we pull in some members?"
	}

	blocks := []slack.Block{
		slack.NewHeaderBlock(
			&slack.TextBlockObject{
				Type: slack.PlainTextType,
				Text: rotaName,
			},
		),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("Duration of an on-call shift: %d week(s)", rotaDuration),
			},
			nil,
			nil,
		),
	}

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

	rotaActionsBlock := slack.NewActionBlock(rotaActions)

	if len(rotaMembers) > 0 {
		if currOnCallMember == "" {
			rotaActionsBlock.Elements.ElementSet = append(
				rotaActionsBlock.Elements.ElementSet,
				&slack.ButtonBlockElement{
					Type:     "button",
					ActionID: UpdateRotaPromptAction,
					Text:     &slack.TextBlockObject{Text: "Update the rota", Type: slack.PlainTextType},
					Style:    slack.StyleDefault,
					Value:    rotaName,
				},
				&slack.ButtonBlockElement{
					Type:     "button",
					ActionID: StartRotaAction,
					Text:     &slack.TextBlockObject{Text: "Start the rota", Type: slack.PlainTextType},
					Style:    slack.StylePrimary,
					Value:    rotaName,
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
					Value:    rotaName,
				},
			)
		}
	}

	blocks = append(blocks, rotaActionsBlock)

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{BlockSet: blocks}

	return &attachment
}

func (c *RotaCommand) upsertRotaCallback(channelId string, userId string, rotaName string, rotaMembers []string, rotaDuration string) error {
	err := c.saveRotaDetails(channelId, rotaName, rotaMembers, rotaDuration)
	if err != nil {
		return err
	}

	rotaDurationAsInt, _ := strconv.Atoi(rotaDuration)

	prompt := c.rotaDetailsPrompt(rotaMembers, "", rotaName, rotaDurationAsInt, "")
	prompt.Color = "#4af030"

	err = c.respondToClient(channelId, userId, prompt)
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) upsertRotaPrompt(channelId string, triggerId string, rotaName string, callbackId string) error {
	var titleTextContent string
	if callbackId == CreateRotaCallback {
		titleTextContent = "Create your rota"
	} else if callbackId == UpdateRotaCallback {
		titleTextContent = "Update your rota"
	}

	titleText := slack.NewTextBlockObject(slack.PlainTextType, titleTextContent, false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false)

	var blockSet []slack.Block
	if callbackId == CreateRotaCallback {
		rotaNameText := slack.NewTextBlockObject(slack.PlainTextType, "Rota Name", false, false)
		rotaNamePlaceholder := slack.NewTextBlockObject(slack.PlainTextType, "New rota name", false, false)
		rotaNameElement := slack.NewPlainTextInputBlockElement(rotaNamePlaceholder, rotaNameAction)
		rotaNameElement.MaxLength = 50
		rotaNameElement.MinLength = 5
		rotaNameInputBlock := slack.NewInputBlock(rotaNameBlock, rotaNameText, rotaNameElement)

		blockSet = append(blockSet, rotaNameInputBlock)
	}

	var initialRotaMembers []string
	if callbackId == UpdateRotaCallback {
		rotaDetails, err := c.getRotaDetails(channelId, rotaName)
		if err != nil {
			return err
		}

		initialRotaMembers = rotaDetails.Members
	}

	rotaMemberSelectionText := slack.NewTextBlockObject(slack.PlainTextType, "Select members of your rota", false, false)
	rotaMemberSelectionElement := &slack.MultiSelectBlockElement{
		Type:         slack.MultiOptTypeUser,
		ActionID:     rotaMembersAction,
		Placeholder:  nil,
		InitialUsers: initialRotaMembers,
	}
	rotaMemberSelectionInputBlock := slack.NewInputBlock(rotaMembersBlock, rotaMemberSelectionText, rotaMemberSelectionElement)

	rotaDurationText := slack.NewTextBlockObject(slack.PlainTextType, "How long is one on-call shift?", false, false)
	rotaDurationOptionBlockObjects := []*slack.OptionBlockObject{
		slack.NewOptionBlockObject("1", slack.NewTextBlockObject(slack.PlainTextType, "1 week", false, false), nil),
		slack.NewOptionBlockObject("2", slack.NewTextBlockObject(slack.PlainTextType, "2 weeks", false, false), nil),
	}
	rotaDurationElement := slack.NewRadioButtonsBlockElement(rotaDurationAction, rotaDurationOptionBlockObjects...)
	// TODO: rotaDurationElement.InitialOption
	rotaDurationInputBlock := slack.NewInputBlock(rotaDurationBlock, rotaDurationText, rotaDurationElement)

	blockSet = append(
		blockSet,
		rotaMemberSelectionInputBlock,
		rotaDurationInputBlock,
	)
	blocks := slack.Blocks{
		BlockSet: blockSet,
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = "modal"
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.CallbackID = callbackId
	modalRequest.ExternalID = channelId

	if callbackId == UpdateRotaCallback {
		modalRequest.PrivateMetadata = rotaName
	}

	_, err := c.client.OpenView(triggerId, modalRequest)
	if err != nil {
		return err
	}

	return nil
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

func (c *RotaCommand) saveRotaDetails(channelId string, rotaName string, rotaMembers []string, rotaDuration string) error {
	var rotaMembersAsAttr []types.AttributeValue
	for _, v := range rotaMembers {
		rotaMembersAsAttr = append(rotaMembersAsAttr, &types.AttributeValueMemberS{Value: v})
	}

	_, err := c.db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(TableName),
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

func (c *RotaCommand) updateOnCallMember(channelId string, rotaName string, newOnCallMember string, startOfShift string, endOfShift string) error {
	_, err := c.db.UpdateItem(context.TODO(), &dynamodb.UpdateItemInput{
		TableName: aws.String(TableName),
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

func (rd *RotaDetails) rotaName() string {
	return rd.Sk
}

func (rd *RotaDetails) generateEndOfShift(startOfShift time.Time) string {
	return formatTime(startOfShift.Add(time.Hour * 168 * time.Duration(rd.Duration)))
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

func formatTime(rawTime time.Time) string {
	return rawTime.Format(time.RFC1123)
}
