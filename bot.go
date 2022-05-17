package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"log"
	"os"
)

type Bot struct {
	socketClient *socketmode.Client
	rotaCommand  *RotaCommand
}

func NewBot(token string, appToken string) *Bot {
	client := slack.New(token, slack.OptionDebug(true), slack.OptionAppLevelToken(appToken))
	db := initDatabase()
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	return &Bot{
		socketClient: socketClient,
		rotaCommand:  NewRotaCommand(db, client),
	}
}

func (b *Bot) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.listenToEvents(ctx)

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
			case SelectRotaAction:
				return b.rotaCommand.PromptRotaDetails(&interaction, action)
			case StartRotaAction:
				return b.rotaCommand.StartRotaPrompt(&interaction, action)
			case StopRotaAction:
				return b.rotaCommand.StopRota()
			case CreateRotaPromptAction:
				return b.rotaCommand.CreateRotaPrompt(&interaction)
			case UpdateRotaPromptAction:
				return b.rotaCommand.UpdateRotaPrompt(&interaction, action)
			}
		}
	case slack.InteractionTypeViewSubmission:
		switch interaction.View.CallbackID {
		case UpdateRotaCallback:
			return b.rotaCommand.UpdateRota(&interaction)
		case CreateRotaCallback:
			return b.rotaCommand.CreateRota(&interaction)
		case StartRotaCallback:
			return b.rotaCommand.StartRota(&interaction)
		}
	}

	return nil
}

func initDatabase() *dynamodb.Client {
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
