package slackclient

import "github.com/slack-go/slack"

type SlackClient interface {
	PostMessage(channelID string, attachment slack.Attachment) (string, string, error)
	PostEphemeral(channelID string, userID string, attachment slack.Attachment) (string, error)
	OpenView(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error)
}

type SlackWrapper struct {
	client *slack.Client
	_      func(channelID string, attachment slack.Attachment) (string, string, error)
	_      func(channelID string, userID string, attachment slack.Attachment) (string, error)
	_      func(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error)
}

func New(client *slack.Client) *SlackWrapper {
	return &SlackWrapper{client: client}
}

func (w *SlackWrapper) PostMessage(channelID string, attachment slack.Attachment) (string, string, error) {
	return w.client.PostMessage(channelID, slack.MsgOptionAttachments(attachment))
}

func (w *SlackWrapper) PostEphemeral(channelID string, userID string, attachment slack.Attachment) (string, error) {
	return w.client.PostEphemeral(channelID, userID, slack.MsgOptionAttachments(attachment))
}

func (w *SlackWrapper) OpenView(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error) {
	return w.client.OpenView(triggerID, view)
}
