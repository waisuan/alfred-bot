package rota

import (
	"fmt"
	"github.com/slack-go/slack"
	"log"
	"strings"
	"time"
)

type Command struct {
	currOnCallMember string
	rotaList         []string
	channelId        string
}

func NewCommand() *Command {
	return &Command{
		currOnCallMember: "",
		rotaList:         []string{},
		channelId:        "C03C9SC7EH1",
	}
}

func (c *Command) Start(client *slack.Client) {
	go func() {
		for {
			for idx, member := range c.rotaList {
				if c.currOnCallMember == member {
					c.currOnCallMember = c.rotaList[(idx+1)%len(c.rotaList)]
					break
				}
			}

			if c.currOnCallMember != "" {
				attachment := slack.Attachment{}
				attachment.Text = fmt.Sprintf("The new on-call person is: %s", atUserId(c.currOnCallMember))
				attachment.Color = "#4af030"
				_, _, err := client.PostMessage(c.channelId, slack.MsgOptionAttachments(attachment))
				if err != nil {
					log.Println(err)
				}
			}

			time.Sleep(30 * time.Second)
		}
	}()
}

func (c *Command) HandlePrompt(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	multiSelect := &slack.MultiSelectBlockElement{
		Type:         slack.MultiOptTypeUser,
		ActionID:     "select_rota_members",
		Placeholder:  &slack.TextBlockObject{Text: "Select members of your rota", Type: slack.PlainTextType},
		InitialUsers: c.rotaList,
	}

	accessory := slack.NewAccessory(multiSelect)

	var currRotaMembersText string
	var currOnCallMemberText string
	if len(c.rotaList) > 0 {
		currRotaMembersText = fmt.Sprintf("Current rota members:\n%s", c.formattedRotaMemberList())

		if c.currOnCallMember != "" {
			currOnCallMemberText = fmt.Sprintf("*The current on-call person is: %s*", atUserId(c.currOnCallMember))
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
			accessory,
		),
	)

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{BlockSet: blocks}

	return attachment, nil
}

func (c *Command) HandleSelection(action *slack.BlockAction, client *slack.Client) error {
	c.rotaList = []string{}
	c.currOnCallMember = ""
	for _, userId := range action.SelectedUsers {
		user, err := client.GetUserInfo(userId)
		if err != nil {
			return err
		}
		c.rotaList = append(c.rotaList, user.ID)
	}

	var postSelectionText string
	if len(c.rotaList) > 0 {
		c.currOnCallMember = c.rotaList[0]
		// attachment.Pretext = fmt.Sprintf("The current on-call person is: %s", atUserId(c.currOnCallMember))
		postSelectionText = fmt.Sprintf("The rota now consists of:\n%s", c.formattedRotaMemberList())
	} else {
		postSelectionText = "There are no members set to the rota."
	}

	multiSelect := &slack.ButtonBlockElement{
		Type:     "button",
		ActionID: "start_rota",
		Text:     &slack.TextBlockObject{Text: "Start the rota", Type: slack.PlainTextType},
	}

	accessory := slack.NewAccessory(multiSelect)

	//slack.NewActionBlock()

	attachment := slack.Attachment{}
	attachment.Color = "#4af030"
	attachment.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: postSelectionText,
				},
				nil,
				accessory,
			),
		},
	}

	_, _, err := client.PostMessage(c.channelId, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return err
	}

	return nil
}

func (c *Command) formattedRotaMemberList() string {
	var formattedUserIds []string
	for _, userId := range c.rotaList {
		formattedUserIds = append(formattedUserIds, fmt.Sprintf("• %s", atUserId(userId)))
	}
	return fmt.Sprintf("%s", strings.Join(formattedUserIds, "\n"))
}

func atUserId(userId string) string {
	return fmt.Sprintf("<@%s>", userId)
}
