package bot

import (
	"alfred-bot/cmd/bot/commands/rotacommand"
	rotaHandler "alfred-bot/cmd/bot/commands/rotacommand/handler"
	"alfred-bot/utils/db"
	"context"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"log"
	"os"
)

type Bot struct {
	socketClient *socketmode.Client
	rotaCommand  *rotacommand.RotaCommand
}

func New(token string, appToken string) *Bot {
	client := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	dbClient := db.Init(db.TableName)
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	return &Bot{
		socketClient: socketClient,
		rotaCommand:  rotacommand.New(rotaHandler.New(dbClient), client),
	}
}

func (b *Bot) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.listenToEvents(ctx)
	b.startBackgroundTasks()

	_ = b.socketClient.Run()
}

func (b *Bot) listenToEvents(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Shutting down...")
				return
			case event := <-b.socketClient.Events:
				switch event.Type {
				case socketmode.EventTypeEventsAPI:
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Could not type cast the event to the EventsAPIEvent: %v\n", event)
						continue
					}
					b.socketClient.Ack(*event.Request)

					err := b.handleEventMessage(eventsAPIEvent)
					if err != nil {
						log.Println(err)
					}
				case socketmode.EventTypeSlashCommand:
					command, ok := event.Data.(slack.SlashCommand)
					if !ok {
						log.Printf("Could not type cast the message to a SlashCommand: %v\n", command)
						continue
					}

					payload, err := b.handleSlashCommand(command)
					if err != nil {
						log.Println(err)
						continue
					}

					b.socketClient.Ack(*event.Request, payload)
				case socketmode.EventTypeInteractive:
					interaction, ok := event.Data.(slack.InteractionCallback)
					if !ok {
						log.Printf("Could not type cast the message to a Interaction callback: %v\n", interaction)
						continue
					}

					err := b.handleInteractionEvent(interaction)
					if err != nil {
						log.Println(err)
						continue
					}

					b.socketClient.Ack(*event.Request)
				}
			}
		}
	}()
}

func (b *Bot) startBackgroundTasks() {
	b.rotaCommand.HandleEndOfOnCallShifts()
}

func (b *Bot) handleEventMessage(event slackevents.EventsAPIEvent) error {
	//switch event.Type {
	//case slackevents.CallbackEvent:
	//	innerEvent := event.InnerEvent
	//	switch ev := innerEvent.Data.(type) {
	//	case *slackevents.MessageEvent:
	//		if ev.Text == "I need help!" && ev.ThreadTimeStamp == "" {
	//			client.PostMessage(ev.Channel, slack.MsgOptionText("Hi, I'm here!", false), slack.MsgOptionTS(ev.TimeStamp))
	//		}
	//	}
	//default:
	//	return errors.New("unsupported event type")
	//}
	return nil
}

func (b *Bot) handleSlashCommand(command slack.SlashCommand) (interface{}, error) {
	switch command.Command {
	case "/rota":
		return b.rotaCommand.Prompt(command)
	}
	return nil, nil
}

func (b *Bot) handleInteractionEvent(interaction slack.InteractionCallback) error {
	log.Println(">>> " + interaction.Type)
	switch interaction.Type {
	case slack.InteractionTypeBlockActions:
		for _, action := range interaction.ActionCallback.BlockActions {
			switch action.ActionID {
			case rotacommand.SelectRotaAction:
				return b.rotaCommand.PromptRotaDetails(&interaction, action)
			case rotacommand.StartRotaAction:
				return b.rotaCommand.StartRotaPrompt(&interaction, action)
			case rotacommand.StopRotaAction:
				return b.rotaCommand.StopRota(&interaction, action)
			case rotacommand.CreateRotaPromptAction:
				return b.rotaCommand.CreateRotaPrompt(&interaction)
			case rotacommand.UpdateRotaPromptAction:
				return b.rotaCommand.UpdateRotaPrompt(&interaction, action)
			}
		}
	case slack.InteractionTypeViewSubmission:
		switch interaction.View.CallbackID {
		case rotacommand.UpdateRotaCallback:
			return b.rotaCommand.UpdateRota(&interaction)
		case rotacommand.CreateRotaCallback:
			return b.rotaCommand.CreateRota(&interaction)
		case rotacommand.StartRotaCallback:
			return b.rotaCommand.StartRota(&interaction)
		}
	}

	return nil
}
