package rotacommand

import (
	"alfred-bot/cmd/bot/commands/rotacommand/handler"
	"alfred-bot/cmd/bot/commands/rotacommand/models/metadata"
	"alfred-bot/cmd/bot/commands/rotacommand/models/rotadetails"
	"alfred-bot/utils/formatter"
	"fmt"
	"github.com/slack-go/slack"
	"log"
	"strconv"
	"time"
)

const (
	StartRotaAction        = "start_rota_prompt"
	StopRotaAction         = "stop_rota"
	SelectRotaAction       = "select_rota"
	UpdateRotaPromptAction = "update_rota_prompt"
	CreateRotaPromptAction = "create_rota_prompt"
	UpdateRotaCallback     = "update_rota"
	CreateRotaCallback     = "create_rota"
	StartRotaCallback      = "start_rota"
	rotaActions            = "rota_actions"
	promptActions          = "prompt_actions"
	rotaNameAction         = "set_rota_name"
	rotaMembersAction      = "select_rota_members"
	rotaDurationAction     = "set_rota_duration"
	rotaOnCallMemberAction = "set_on_call_member"
	rotaNameBlock          = "rota_name"
	rotaMembersBlock       = "rota_members"
	rotaDurationBlock      = "rota_duration"
	rotaOnCallMemberBlock  = "on_call_member"
)

type RotaCommand struct {
	handler *handler.RotaHandler
	client  *slack.Client
}

func New(handler *handler.RotaHandler, client *slack.Client) *RotaCommand {
	return &RotaCommand{
		handler: handler,
		client:  client,
	}
}

func (c *RotaCommand) HandleEndOfOnCallShifts() {
	go func() {
		for {
			rotas, err := c.handler.GetEndingOnCallShifts()
			if err != nil {
				log.Println(err)
			}

			for _, v := range rotas {
				log.Println(v)

				var nextOnCallMember string
				for i, m := range v.Members {
					if m == v.CurrOnCallMember {
						nextOnCallMember = v.Members[(i+1)%len(v.Members)]
					}
				}
				startOfShift := time.Now()
				endOfShift := rotadetails.GenerateEndOfShift(startOfShift, v.Duration)
				err := c.handler.UpdateOnCallMember(v.Pk, v.Sk, nextOnCallMember, formatter.FormatTime(startOfShift), endOfShift)
				if err != nil {
					log.Println(fmt.Sprintf("Could not update rota shift for %v (%v): %v", v.Sk, v.Pk, err))
				} else {
					attachment := slack.Attachment{}
					attachment.Text = fmt.Sprintf("[%v] %s now on duty!", v.Sk, formatter.AtUserId(nextOnCallMember))
					attachment.Color = "#4af030"
					_, _, err = c.client.PostMessage(v.Pk, slack.MsgOptionAttachments(attachment))
					if err != nil {
						log.Println(err)
					}
				}
			}

			time.Sleep(time.Minute)
		}
	}()
}

func (c *RotaCommand) StartRotaPrompt(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	channelId := interaction.Channel.ID
	rotaName := action.Value

	rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	var unableToStartRotaErr string

	rotaMembers := rotaDetails.Members
	if len(rotaMembers) == 0 {
		unableToStartRotaErr = "Sorry, I can't start an empty rota!"
	}

	if rotaDetails.Duration == 0 {
		unableToStartRotaErr = "Sorry, I can't start a rota without a shift duration!"
	}

	if rotaDetails.CurrOnCallMember != "" {
		unableToStartRotaErr = fmt.Sprintf("[%s] %s is already currently on duty!", rotaName, formatter.AtUserId(rotaDetails.CurrOnCallMember))
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

	titleText := slack.NewTextBlockObject(slack.PlainTextType, "Start a shift", false, false)
	closeText := slack.NewTextBlockObject(slack.PlainTextType, "Close", false, false)
	submitText := slack.NewTextBlockObject(slack.PlainTextType, "Save", false, false)

	onCallMemberText := slack.NewTextBlockObject(slack.PlainTextType, "Who should be on duty for this shift?", false, false)
	onCallOptionBlockObjects := make([]*slack.OptionBlockObject, 0, len(rotaMembers))
	for _, v := range rotaMembers {
		optionText := slack.NewTextBlockObject(slack.PlainTextType, formatter.AtUserId(v), false, false)
		onCallOptionBlockObjects = append(onCallOptionBlockObjects, slack.NewOptionBlockObject(v, optionText, nil))
	}
	onCallMemberElement := slack.NewOptionsSelectBlockElement(slack.OptTypeStatic, nil, rotaOnCallMemberAction, onCallOptionBlockObjects...)
	onCallMemberInputBlock := slack.NewInputBlock(rotaOnCallMemberBlock, onCallMemberText, onCallMemberElement)

	startOfShiftTime := time.Now()
	endOfShiftTime := rotadetails.GenerateEndOfShift(startOfShiftTime, rotaDetails.Duration)
	shiftDetailsBlock := slack.NewSectionBlock(
		&slack.TextBlockObject{
			Type: slack.MarkdownType,
			Text: fmt.Sprintf("*Their shift will end on: %v*", endOfShiftTime),
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

	modalRequest.PrivateMetadata, err = metadata.GenerateCommandMetadata(
		channelId,
		rotaName,
		formatter.FormatTime(startOfShiftTime),
		endOfShiftTime,
	)
	if err != nil {
		return err
	}

	_, err = c.client.OpenView(interaction.TriggerID, modalRequest)
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) StopRota(interaction *slack.InteractionCallback, action *slack.BlockAction) error {
	channelId := interaction.Channel.ID
	userId := interaction.User.ID
	rotaName := action.Value

	rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	if rotaDetails.CurrOnCallMember == "" {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("[%v] Can't stop a shift that has yet to start.", rotaName)
		attachment.Color = "#f0303a"
		err := c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	err = c.handler.UpdateOnCallMember(channelId, rotaName, "", "", "")
	if err != nil {
		return err
	}

	attachment := slack.Attachment{}
	attachment.Text = fmt.Sprintf("[%v] %s is now off duty!", rotaName, formatter.AtUserId(rotaDetails.CurrOnCallMember))
	attachment.Color = "#4af030"
	_, _, err = c.client.PostMessage(channelId, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return err
	}

	return nil
}

func (c *RotaCommand) Prompt(command slack.SlashCommand) (interface{}, error) {
	rotaNames, err := c.handler.GetRotaNames(command.ChannelID)
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

	rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
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
	userId := interaction.User.ID
	rotaName := action.Value

	rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	if rotaDetails.CurrOnCallMember != "" {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("[%v] Can't update rota whilst someone is on duty.", rotaName)
		attachment.Color = "#f0303a"
		err := c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	return c.upsertRotaPrompt(channelId, triggerId, rotaName, UpdateRotaCallback)
}

func (c *RotaCommand) CreateRota(interaction *slack.InteractionCallback) error {
	metadata, err := metadata.UnpackCommandMetadata(interaction.View.PrivateMetadata)
	if err != nil {
		return err
	}

	view := interaction.View
	userId := interaction.User.ID
	channelId := metadata.ChannelId
	inputs := view.State.Values
	rotaName := inputs[rotaNameBlock][rotaNameAction].Value
	rotaMembers := inputs[rotaMembersBlock][rotaMembersAction].SelectedUsers
	rotaDuration := inputs[rotaDurationBlock][rotaDurationAction].SelectedOption.Value

	rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
	if err != nil {
		return err
	}

	if rotaDetails.Pk != "" {
		attachment := slack.Attachment{}
		attachment.Text = fmt.Sprintf("Oops, %s already exists!", rotaName)
		attachment.Color = "#f0303a"

		err = c.respondToClient(channelId, userId, &attachment)
		if err != nil {
			return err
		}

		return nil
	}

	return c.upsertRotaCallback(channelId, userId, rotaName, rotaMembers, rotaDuration)
}

func (c *RotaCommand) UpdateRota(interaction *slack.InteractionCallback) error {
	metadata, err := metadata.UnpackCommandMetadata(interaction.View.PrivateMetadata)
	if err != nil {
		return err
	}

	view := interaction.View
	userId := interaction.User.ID
	channelId := metadata.ChannelId
	rotaName := metadata.RotaName
	inputs := view.State.Values
	rotaMembers := inputs[rotaMembersBlock][rotaMembersAction].SelectedUsers
	rotaDuration := inputs[rotaDurationBlock][rotaDurationAction].SelectedOption.Value

	return c.upsertRotaCallback(channelId, userId, rotaName, rotaMembers, rotaDuration)
}

func (c *RotaCommand) StartRota(interaction *slack.InteractionCallback) error {
	view := interaction.View
	inputs := view.State.Values
	onCallMember := inputs[rotaOnCallMemberBlock][rotaOnCallMemberAction].SelectedOption.Value

	metadata, err := metadata.UnpackCommandMetadata(view.PrivateMetadata)
	if err != nil {
		return err
	}

	err = c.handler.UpdateOnCallMember(metadata.ChannelId, metadata.RotaName, onCallMember, metadata.StartOfShift, metadata.EndOfShift)
	if err != nil {
		return err
	}

	attachment := slack.Attachment{}
	attachment.Text = fmt.Sprintf("[%v] %s is now on duty!", metadata.RotaName, formatter.AtUserId(onCallMember))
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
		currRotaMembersText = fmt.Sprintf("Current rota members:\n%s", formatter.RotaMembersAsString(rotaMembers))

		if currOnCallMember != "" {
			currOnCallMemberText = fmt.Sprintf("*%s is currently on duty (shift ends at %v)*", formatter.AtUserId(currOnCallMember), endOfShift)
		} else {
			currOnCallMemberText = "*No one is currently on duty.*"
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
				Text: fmt.Sprintf("Duration of a rota shift: %d week(s)", rotaDuration),
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
					Text:     &slack.TextBlockObject{Text: "Start shift", Type: slack.PlainTextType},
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
					Text:     &slack.TextBlockObject{Text: "Stop shift", Type: slack.PlainTextType},
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
	err := c.handler.SaveRotaDetails(channelId, rotaName, rotaMembers, rotaDuration)
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
		rotaDetails, err := c.handler.GetRotaDetails(channelId, rotaName)
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

	rotaDurationText := slack.NewTextBlockObject(slack.PlainTextType, "How long is one rota shift?", false, false)
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

	metadata, err := metadata.GenerateCommandMetadata(
		channelId,
		rotaName,
		"",
		"",
	)
	if err != nil {
		return err
	}

	modalRequest.PrivateMetadata = metadata

	_, err = c.client.OpenView(triggerId, modalRequest)
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
