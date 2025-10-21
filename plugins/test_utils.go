package plugins

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stretchr/testify/mock"
)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockHTTPClient) Head(url string) (*http.Response, error) {
	args := m.Called(url)
	return args.Get(0).(*http.Response), args.Error(1)
}

// Helper function to create fresh HTTP response
func createHTTPResponse(html string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(html)),
	}
}

// Helper function for setting up HTTP response sequences
func setupHTTPSequence(mock *MockHTTPClient, url string, responses []string) {
	for _, response := range responses {
		mock.On("Get", url).Return(createHTTPResponse(response), nil).Once()
	}
}

type MockClock struct {
	now time.Time
}

func (m *MockClock) Now() time.Time {
	return m.now
}

func (m *MockClock) Advance(d time.Duration) {
	m.now = m.now.Add(d)
}

type MockDiscordClient struct {
	mock.Mock
	SentMessages []DiscordMessage // Capture sent messages
}

type DiscordMessage struct {
	Text   string
	Name   string
	Avatar string
	Embed  interface{}
}

func (m *MockDiscordClient) Send(text, name, avatar string, embed interface{}) error {
	// Capture the message
	m.SentMessages = append(m.SentMessages, DiscordMessage{
		Text:   text,
		Name:   name,
		Avatar: avatar,
		Embed:  embed,
	})

	args := m.Called(text, name, avatar, embed)
	return args.Error(0)
}

func (m *MockDiscordClient) SendWithCustomAvatar(text, name, avatarURL string, embed interface{}) error {
	// Capture the message
	m.SentMessages = append(m.SentMessages, DiscordMessage{
		Text:   text,
		Name:   name,
		Avatar: avatarURL,
		Embed:  embed,
	})

	args := m.Called(text, name, avatarURL, embed)
	return args.Error(0)
}
