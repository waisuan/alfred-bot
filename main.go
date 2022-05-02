package main

import (
	"alfred-bot/commands/rota"
	"context"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"log"
	"os"
	"strings"
	"time"
)

var rotaCommand = rota.NewCommand()

func main() {
	_ = godotenv.Load(".env")

	token := os.Getenv("SLACK_AUTH_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	client := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go listenToEvents(ctx, client, socketClient)

	_ = socketClient.Run()
}

func listenToEvents(ctx context.Context, client *slack.Client, socketClient *socketmode.Client) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return
		case event := <-socketClient.Events:
			switch event.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
				if !ok {
					log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
					continue
				}
				socketClient.Ack(*event.Request)

				err := handleEventMessage(eventsAPIEvent, client)
				if err != nil {
					log.Println(err)
				}
			case socketmode.EventTypeSlashCommand:
				command, ok := event.Data.(slack.SlashCommand)
				if !ok {
					log.Printf("Could not type cast the message to a SlashCommand: %v\n", command)
					continue
				}

				payload, err := handleSlashCommand(command, client)
				if err != nil {
					log.Println(err)
					continue
				}

				socketClient.Ack(*event.Request, payload)
			case socketmode.EventTypeInteractive:
				interaction, ok := event.Data.(slack.InteractionCallback)
				if !ok {
					log.Printf("Could not type cast the message to a Interaction callback: %v\n", interaction)
					continue
				}

				err := handleInteractionEvent(interaction, client)
				if err != nil {
					log.Println(err)
					continue
				}

				socketClient.Ack(*event.Request)
			}
		}
	}
}

func handleEventMessage(event slackevents.EventsAPIEvent, client *slack.Client) error {
	switch event.Type {
	case slackevents.CallbackEvent:
		innerEvent := event.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			err := handleAppMentionEvent(ev, client)
			if err != nil {
				return err
			}
		case *slackevents.MessageEvent:
			if ev.Text == "I need help!" && ev.ThreadTimeStamp == "" {
				client.PostMessage(ev.Channel, slack.MsgOptionText("Hi, I'm here!", false), slack.MsgOptionTS(ev.TimeStamp))
			}
		}
	default:
		return errors.New("unsupported event type")
	}
	return nil
}

func handleAppMentionEvent(event *slackevents.AppMentionEvent, client *slack.Client) error {
	user, err := client.GetUserInfo(event.User)
	if err != nil {
		return err
	}

	attachment := slack.Attachment{}
	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initializer",
			Value: user.Name,
		},
	}

	text := strings.ToLower(event.Text)
	if strings.Contains(text, "hello") {
		attachment.Text = fmt.Sprintf("Hello %s", user.Name)
		attachment.Pretext = "Greetings"
		attachment.Color = "#4af030"
	} else {
		attachment.Text = fmt.Sprintf("How can I help you %s?", user.Name)
		attachment.Pretext = "How can I be of service"
		attachment.Color = "#3d3d3d"
	}

	_, _, err = client.PostMessage(event.Channel, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

func handleSlashCommand(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	switch command.Command {
	case "/hello":
		return nil, handleHelloCommand(command, client)
	case "/question":
		return handleQuestionCommand(command, client)
	case "/rota":
		return rotaCommand.HandlePrompt(command, client)
	}
	return nil, nil
}

func handleHelloCommand(command slack.SlashCommand, client *slack.Client) error {
	attachment := slack.Attachment{}
	attachment.Fields = []slack.AttachmentField{
		{
			Title: "Date",
			Value: time.Now().String(),
		}, {
			Title: "Initializer",
			Value: command.UserName,
		},
	}

	attachment.Text = fmt.Sprintf("Hello %s", command.Text)
	attachment.Color = "#4af030"

	_, _, err := client.PostMessage(command.ChannelID, slack.MsgOptionAttachments(attachment))
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}

	return nil
}

func handleQuestionCommand(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	checkbox := slack.NewCheckboxGroupsBlockElement("answer",
		slack.NewOptionBlockObject("yes", &slack.TextBlockObject{Text: "Yes", Type: slack.MarkdownType}, &slack.TextBlockObject{Text: "Did you Enjoy it?", Type: slack.MarkdownType}),
		slack.NewOptionBlockObject("no", &slack.TextBlockObject{Text: "No", Type: slack.MarkdownType}, &slack.TextBlockObject{Text: "Did you Dislike it?", Type: slack.MarkdownType}),
	)
	accessory := slack.NewAccessory(checkbox)

	attachment := slack.Attachment{}
	attachment.Blocks = slack.Blocks{
		BlockSet: []slack.Block{
			slack.NewSectionBlock(
				&slack.TextBlockObject{
					Type: slack.MarkdownType,
					Text: "Did you think this article was helpful?",
				},
				nil,
				accessory,
			),
		},
	}

	attachment.Text = "Rate the tutorial"
	attachment.Color = "#4af030"

	return attachment, nil
}

func handleInteractionEvent(interaction slack.InteractionCallback, client *slack.Client) error {
	switch interaction.Type {
	case slack.InteractionTypeBlockActions:
		for _, action := range interaction.ActionCallback.BlockActions {
			log.Printf("%+v", action)
			switch action.ActionID {
			case "select_rota_members":
				return rotaCommand.HandleSelection(action, client)
			}
		}
	}

	return nil
}
