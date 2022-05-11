package main

import (
	"alfred-bot/commands/rota"
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/joho/godotenv"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"log"
	"os"
)

var rotaCommand *rota.Command

func main() {
	db := initDatabase()
	rotaCommand = rota.NewCommand(db)

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

func handleSlashCommand(command slack.SlashCommand, client *slack.Client) (interface{}, error) {
	switch command.Command {
	case "/rota":
		return rotaCommand.HandlePrompt(command, client)
	}
	return nil, nil
}

func handleInteractionEvent(interaction slack.InteractionCallback, client *slack.Client) error {
	switch interaction.Type {
	case slack.InteractionTypeBlockActions:
		for _, action := range interaction.ActionCallback.BlockActions {
			log.Printf("%+v", action)
			switch action.ActionID {
			case rota.SelectRotaMembersAction:
				return rotaCommand.HandleSelection(interaction.Channel, action, client)
			case rota.StartRotaAction:
				rotaCommand.StartRota(action, client)
				return nil
			case rota.StopRotaAction:
				return rotaCommand.StopRota(client)
			}
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

	_, err = svc.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{TableName: aws.String(rota.TableName)})
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
			TableName:   aws.String(rota.TableName),
			BillingMode: types.BillingModePayPerRequest,
		})
		if err != nil {
			panic(err)
		}
	}

	return svc
}
