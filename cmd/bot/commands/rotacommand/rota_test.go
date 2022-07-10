package rotacommand

import (
	"alfred-bot/cmd/bot/commands/rotacommand/models/rotadetails"
	"alfred-bot/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/slack-go/slack"
	"testing"
)

const (
	testChannelId = "dummy_channel"
	testRotaName  = "dummy_rota"
)

type MockSlackClient struct {
	PostMessageStub   func(channelID string, attachment slack.Attachment) (string, string, error)
	PostEphemeralStub func(channelID string, userID string, attachment slack.Attachment) (string, error)
	OpenViewStub      func(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error)
	Inbox             []string
}

func (m *MockSlackClient) PostMessage(channelID string, attachment slack.Attachment) (string, string, error) {
	return "", "", nil
}

func (m *MockSlackClient) PostEphemeral(channelID string, userID string, attachment slack.Attachment) (string, error) {
	return "", nil
}

func (m *MockSlackClient) OpenView(triggerID string, view slack.ModalViewRequest) (*slack.ViewResponse, error) {
	m.Inbox = append(
		m.Inbox,
		string(view.Type),
		view.CallbackID,
		view.PrivateMetadata,
		view.Title.Text,
	)
	return nil, nil
}

type MockRotaHandler struct {
	_ func(channelId string) ([]string, error)
	_ func(channelId string, rotaName string) (*rotadetails.RotaDetails, error)
	_ func() ([]*rotadetails.RotaDetails, error)
	_ func(channelId string, rotaName string, rotaMembers []string, rotaDuration string) error
	_ func(channelId string, rotaName string, newOnCallMember string, startOfShift string, endOfShift string) error
}

func (r *MockRotaHandler) GetRotaNames(channelId string) ([]string, error) {
	return nil, nil
}

func (r *MockRotaHandler) GetRotaDetails(channelId string, rotaName string) (*rotadetails.RotaDetails, error) {
	if channelId == testChannelId && rotaName == testRotaName {
		return &rotadetails.RotaDetails{
			Pk:               testChannelId,
			Sk:               testRotaName,
			Members:          []string{"Evan", "Sia", "Wai", "Suan"},
			CurrOnCallMember: "",
			Duration:         1,
			StartOfShift:     "",
			EndOfShift:       "",
		}, nil
	}
	return nil, nil
}

func (r *MockRotaHandler) GetEndingOnCallShifts() ([]*rotadetails.RotaDetails, error) {
	return nil, nil
}

func (r *MockRotaHandler) SaveRotaDetails(channelId string, rotaName string, rotaMembers []string, rotaDuration string) error {
	return nil
}

func (r *MockRotaHandler) UpdateOnCallMember(channelId string, rotaName string, newOnCallMember string, startOfShift string, endOfShift string) error {
	return nil
}

func TestRota(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RotaCommand Suite")
}

var _ = BeforeSuite(func() {
	config.BootstrapEnv(true)
})

var _ = Describe("RotaCommand", func() {
	Describe("StartRotaPrompt", func() {
		It("When given an existing rota", func() {
			handler := new(MockRotaHandler)
			mockSlackClient := &MockSlackClient{
				Inbox: []string{},
			}

			rotaCommand := New(handler, mockSlackClient)

			channel := slack.Channel{}
			channel.ID = testChannelId

			interaction := &slack.InteractionCallback{
				User:    slack.User{},
				Channel: channel,
			}

			action := &slack.BlockAction{Value: testRotaName}

			err := rotaCommand.StartRotaPrompt(interaction, action)
			Expect(err).To(BeNil())
			Expect(len(mockSlackClient.Inbox)).ToNot(Equal(0))
			Expect(mockSlackClient.Inbox[0]).To(Equal("modal"))
			Expect(mockSlackClient.Inbox[1]).To(Equal(StartRotaCallback))
			Expect(mockSlackClient.Inbox[2]).ToNot(Equal(""))
			Expect(mockSlackClient.Inbox[3]).To(Equal("Start a shift"))
		})
	})
})
