package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock HTTP client
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
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

// Progress bar builder for creating test HTML
type ProgressBarBuilder struct {
	bars []struct {
		title string
		value int
	}
}

func NewProgressBarBuilder() *ProgressBarBuilder {
	return &ProgressBarBuilder{}
}

func (b *ProgressBarBuilder) AddBar(title string, value int) *ProgressBarBuilder {
	b.bars = append(b.bars, struct {
		title string
		value int
	}{title, value})
	return b
}

func (b *ProgressBarBuilder) Build() string {
	html := `<html><body>`
	for _, bar := range b.bars {
		html += fmt.Sprintf(`
	<div class="progress-item-template-123">
		<div class="progress-title-template-456">%s</div>
		<div class="progress-percent-template-789">%d%%</div>
	</div>`, bar.title, bar.value)
	}
	html += `
	</body></html>`
	return html
}

// Offset builder for creating test offsets
type OffsetBuilder struct {
	progress       []Progress
	pendingChange  bool
	lastChangeTime *time.Time
	pendingDiff    []ProgressDiff
}

func NewOffsetBuilder() *OffsetBuilder {
	return &OffsetBuilder{}
}

func (b *OffsetBuilder) AddProgress(title string, value int) *OffsetBuilder {
	b.progress = append(b.progress, Progress{Title: title, Value: value})
	return b
}

func (b *OffsetBuilder) WithPendingChange(pending bool) *OffsetBuilder {
	b.pendingChange = pending
	return b
}

func (b *OffsetBuilder) WithLastChangeTime(t time.Time) *OffsetBuilder {
	b.lastChangeTime = &t
	return b
}

func (b *OffsetBuilder) WithPendingDiff(diff []ProgressDiff) *OffsetBuilder {
	b.pendingDiff = diff
	return b
}

func (b *OffsetBuilder) Build() ProgressOffset {
	return ProgressOffset{
		PublishedState: b.progress,
		ObservedState:  b.progress, // Default to same as published
		DebounceStart:  b.lastChangeTime,
	}
}

// Mock Discord client
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

// PS is a helper function to create Progress instances for tests
func PS(title string, value int) Progress {
	return Progress{Title: title, Value: value}
}

// ScenarioBuilder for creating test scenarios with fluent API
type ScenarioBuilder struct {
	name           string
	debounceDelay  time.Duration
	responses      []string
	sleepDuration  time.Duration
	expectedPosts  int
	expectedFinal  []Progress
	initialOffset  []Progress
	description    string
}

func scenario(name string) *ScenarioBuilder {
	return &ScenarioBuilder{name: name}
}

func (s *ScenarioBuilder) Debounce(delay time.Duration) *ScenarioBuilder {
	s.debounceDelay = delay
	return s
}

func (s *ScenarioBuilder) NoDebounce() *ScenarioBuilder {
	s.debounceDelay = 0
	return s
}

func (s *ScenarioBuilder) ResponseSpecs(specs ...[]Progress) *ScenarioBuilder {
	s.responses = make([]string, len(specs))
	for i, spec := range specs {
		builder := NewProgressBarBuilder()
		for _, p := range spec {
			builder.AddBar(p.Title, p.Value)
		}
		s.responses[i] = builder.Build()
	}
	return s
}

func (s *ScenarioBuilder) Sleep(duration time.Duration) *ScenarioBuilder {
	s.sleepDuration = duration
	return s
}

func (s *ScenarioBuilder) ExpectPosts(count int) *ScenarioBuilder {
	s.expectedPosts = count
	return s
}

func (s *ScenarioBuilder) InitialProgress(title string, value int) *ScenarioBuilder {
	s.initialOffset = []Progress{{Title: title, Value: value}}
	return s
}

func (s *ScenarioBuilder) InitialProgressMultiple(progress ...Progress) *ScenarioBuilder {
	s.initialOffset = progress
	return s
}

func (s *ScenarioBuilder) FinalProgress(title string, value int) *ScenarioBuilder {
	s.expectedFinal = []Progress{{Title: title, Value: value}}
	return s
}

func (s *ScenarioBuilder) FinalProgressMultiple(progress ...Progress) *ScenarioBuilder {
	s.expectedFinal = progress
	return s
}

func (s *ScenarioBuilder) Description(desc string) *ScenarioBuilder {
	s.description = desc
	return s
}

func (s *ScenarioBuilder) Build() struct {
	name           string
	debounceDelay  time.Duration
	responses      []string
	sleepDuration  time.Duration
	expectedPosts  int
	expectedFinal  []Progress
	initialOffset  []Progress
	description    string
} {
	return struct {
		name           string
		debounceDelay  time.Duration
		responses      []string
		sleepDuration  time.Duration
		expectedPosts  int
		expectedFinal  []Progress
		initialOffset  []Progress
		description    string
	}{
		name:           s.name,
		debounceDelay:  s.debounceDelay,
		responses:      s.responses,
		sleepDuration:  s.sleepDuration,
		expectedPosts:  s.expectedPosts,
		expectedFinal:  s.expectedFinal,
		initialOffset:  s.initialOffset,
		description:    s.description,
	}
}

func validateDiscordMessages(t *testing.T, messages []DiscordMessage, expectedPosts int, initialOffset, expectedFinal []Progress) {
	assert.Len(t, messages, expectedPosts, "Should send expected number of messages")
	
	if expectedPosts == 0 {
		return
	}
	
	validateBasicMessage(t, messages[0], initialOffset, expectedFinal)
}

func validateProgressTag(t *testing.T, description string, progress Progress, initialOffset []Progress) {
	// Find initial value for this progress bar
	var initialValue int
	var existed bool
	for _, p := range initialOffset {
		if p.Title == progress.Title {
			initialValue = p.Value
			existed = true
			break
		}
	}
	
	if !existed {
		// New progress bar should have [New] tag
		assert.Contains(t, description, fmt.Sprintf("[New] %s", progress.Title))
	} else if initialValue != progress.Value {
		// Changed progress bar should have [Changed] tag  
		assert.Contains(t, description, fmt.Sprintf("[Changed] %s", progress.Title))
	}
	// Unchanged bars have no tag requirement (just title + percentage)
}

func validateBasicMessage(t *testing.T, msg DiscordMessage, initialOffset, expectedProgress []Progress) {
	assert.Equal(t, "Progress updated!", msg.Text)
	assert.Equal(t, "Progress Updates", msg.Name)
	assert.Equal(t, "dragonsteel", msg.Avatar)
	
	description := getEmbedDescription(t, msg)
	for _, progress := range expectedProgress {
		assert.Contains(t, description, progress.Title)
		assert.Contains(t, description, fmt.Sprintf("%d%%", progress.Value))
		
		// Validate the correct tag is present
		validateProgressTag(t, description, progress, initialOffset)
	}
}

func getEmbedDescription(t *testing.T, msg DiscordMessage) string {
	embed, ok := msg.Embed.(map[string]interface{})
	assert.True(t, ok, "Embed should be a map")
	description, ok := embed["description"].(string)
	assert.True(t, ok, "Description should be a string")
	return description
}

func TestProgressPlugin_Check(t *testing.T) {
	// Build HTML responses using builder
	htmlResponse := NewProgressBarBuilder().
		AddBar("Project A", 75).
		AddBar("Project B", 50).
		Build()

	// Setup mocks
	mockHTTP := &MockHTTPClient{}
	mockDiscord := &MockDiscordClient{}

	// Mock HTTP responses using sequence helper
	setupHTTPSequence(mockHTTP, "https://test.com", []string{
		htmlResponse, // First call
		htmlResponse, // Second call
	})

	// Mock Discord send (should be called when changes detected - twice in this test)
	mockDiscord.On("Send", "Progress updated!", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(2)

	// Setup plugin
	plugin := ProgressPlugin{
		Url:     "https://test.com",
		Message: "Progress updated!",
	}

	ctx := context.Background()
	pluginContext := PluginContext{
		Discord:    mockDiscord,
		Info:       log.New(os.Stdout, "", 0),
		Error:      log.New(os.Stderr, "", 0),
		Context:    &ctx,
		HTTPClient: mockHTTP,
	}

	// Test 1: First run with no offset (should store progress, no message)
	offset1, err := plugin.Check(nil, pluginContext)
	assert.NoError(t, err)

	// Verify progress was stored
	progressOffset, ok := offset1.(ProgressOffset)
	assert.True(t, ok, "Expected ProgressOffset type")
	assert.Len(t, progressOffset.PublishedState, 2)
	assert.Equal(t, "Project A", progressOffset.PublishedState[0].Title)
	assert.Equal(t, 75, progressOffset.PublishedState[0].Value)
	assert.Equal(t, "Project B", progressOffset.PublishedState[1].Title)
	assert.Equal(t, 50, progressOffset.PublishedState[1].Value)

	// Test 2: Run with different previous progress (should detect changes and send message)
	oldOffset := NewOffsetBuilder().
		AddProgress("Project A", 60). // Different from 75%
		AddProgress("Project B", 50). // Same as 50%
		Build()

	offset2, err := plugin.Check(oldOffset, pluginContext)
	assert.NoError(t, err)

	// Verify new progress was stored
	finalOffset, ok := offset2.(ProgressOffset)
	assert.True(t, ok, "Expected ProgressOffset type")
	assert.Len(t, finalOffset.PublishedState, 2)
	assert.Equal(t, "Project A", finalOffset.PublishedState[0].Title)
	assert.Equal(t, 75, finalOffset.PublishedState[0].Value)
	assert.Equal(t, "Project B", finalOffset.PublishedState[1].Title)
	assert.Equal(t, 50, finalOffset.PublishedState[1].Value)
	
	// Verify no pending changes (immediate posting, no debounce)
	assert.False(t, finalOffset.IsDebouncing())
	assert.Nil(t, finalOffset.DebounceStart)

	// Verify mocks were called as expected
	mockHTTP.AssertExpectations(t)
	mockDiscord.AssertExpectations(t)
}

func TestDiff(t *testing.T) {
	// Test no changes
	old := []Progress{
		{Title: "Project A", Value: 50},
		{Title: "Project B", Value: 75},
	}
	new := []Progress{
		{Title: "Project A", Value: 50},
		{Title: "Project B", Value: 75},
	}

	result := diff(old, new)
	assert.Nil(t, result, "Expected no differences for identical progress")

	// Test changes
	newChanged := []Progress{
		{Title: "Project A", Value: 60}, // Changed
		{Title: "Project B", Value: 75}, // Same
	}

	result = diff(old, newChanged)
	assert.NotNil(t, result, "Expected differences to be detected")
	assert.Len(t, result, 2)

	// Check first item changed
	assert.Equal(t, 50, result[0].OldValue)
	assert.Equal(t, 60, result[0].Value)
	assert.Equal(t, "Project A", result[0].Title)

	// Check second item unchanged
	assert.Equal(t, 75, result[1].OldValue)
	assert.Equal(t, 75, result[1].Value)
	assert.Equal(t, "Project B", result[1].Title)
}

func TestProgressPlugin_Scenarios(t *testing.T) {
	tests := []struct {
		name           string
		debounceDelay  time.Duration
		responses      []string
		sleepDuration  time.Duration
		expectedPosts  int
		expectedFinal  []Progress
		initialOffset  []Progress
		description    string
	}{
		scenario("Multiple rapid changes").
			Debounce(10 * time.Millisecond).
			ResponseSpecs(
				[]Progress{PS("Project A", 75)}, // Starts debounce
				[]Progress{PS("Project A", 85)}, // Resets debounce
				[]Progress{PS("Project A", 75)}, // Resets debounce
				[]Progress{PS("Project A", 75)}, // Continues debounce time
				[]Progress{PS("Project A", 75)}, // Continues debounce time
				[]Progress{PS("Project A", 75)}, // Debounce time elapses (post)
			).
			Sleep(4 * time.Millisecond).
			ExpectPosts(1).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("75% -> 85% -> 75% -> 75% (stable, posts final)").
			Build(),

		scenario("Zero debounce").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project A", 75)}, // No debounce, post immediately
			).
			ExpectPosts(1).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("Should work like legacy mode").
			Build(),
		scenario("Very short debounce").
			Debounce(1 * time.Millisecond).
			ResponseSpecs(
				[]Progress{PS("Project A", 75)}, // Starts debounce
				[]Progress{PS("Project A", 75)}, // Debounce elapsed, post change
			).
			Sleep(2 * time.Millisecond).
			ExpectPosts(1).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("Timing edge case").
			Build(),
		scenario("Revert to original").
			Debounce(10 * time.Millisecond).
			ResponseSpecs(
				[]Progress{PS("Project A", 75), PS("Project B", 30)}, // Starts debounce
				[]Progress{PS("Project A", 50), PS("Project B", 30)}, // Debounce elapsed, no change between posted so no new post
			).
			Sleep(15 * time.Millisecond).
			ExpectPosts(0).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			Description("No post when back to original").
			Build(),
		scenario("Multiple bars with debounce").
			Debounce(10 * time.Millisecond).
			ResponseSpecs(
				[]Progress{PS("Project A", 75), PS("Project B", 30)}, // Starts debounce
				[]Progress{PS("Project A", 75), PS("Project B", 60)}, // Resets debounce (change detected from last observed state)
				[]Progress{PS("Project A", 90), PS("Project B", 60)}, // Resets debounce (change detected from last observed state)
				[]Progress{PS("Project A", 90), PS("Project B", 60)}, // Continues debounce
				[]Progress{PS("Project A", 90), PS("Project B", 60)}, // Debounce elapsed, post change
			).
			Sleep(9 * time.Millisecond).
			ExpectPosts(1).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 90),
				PS("Project B", 60),
			).
			Description("Bar A changes → Bar B changes → Bar A changes again → stable").
			Build(),
		scenario("New progress bar appears").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project A", 50), PS("Project B", 30), PS("Project C", 80)}, // No debounce, post immediately
			).
			ExpectPosts(1).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
				PS("Project C", 80),
			).
			Description("New bar detected as change").
			Build(),
		scenario("Multiple bars mixed changes").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project A", 75), PS("Project B", 60)}, // No debounce, post immediately
			).
			ExpectPosts(1).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 75),
				PS("Project B", 60),
			).
			Description("Both bars change values").
			Build(),
		scenario("Same bars different order").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project B", 30), PS("Project A", 50)}, // No debounce, post immediately
			).
			ExpectPosts(0).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			Description("Order independence - no changes").
			Build(),
		scenario("Progress bar removal validation").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project A", 50), PS("Project B", 30)}, // No debounce, post immediately
				[]Progress{PS("Project A", 50)}, // No debounce, does not post because removing a bar does not cause a post if there are no other changes
			).
			ExpectPosts(1).
			InitialProgressMultiple(
				PS("Project A", 50),
			).
			FinalProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			Description("Removal preserves old state (demonstrates current behavior)").
			Build(),
		scenario("Remove then add different bar").
			NoDebounce().
			ResponseSpecs(
				[]Progress{PS("Project A", 50)}, // No debounce, post immediately
				[]Progress{PS("Project A", 50), PS("Project C", 80)}, // No debounce, post immediately
			).
			ExpectPosts(1).
			InitialProgressMultiple(
				PS("Project A", 50),
				PS("Project B", 30),
			).
			FinalProgressMultiple(
				PS("Project A", 50),
				PS("Project C", 80),
			).
			Description("Remove bar (no update) then add different bar (updates state)").
			Build(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockHTTP := &MockHTTPClient{}
			mockDiscord := &MockDiscordClient{}

			// Setup HTTP sequence if we have responses
			if len(tt.responses) > 0 {
				responses := tt.responses
				setupHTTPSequence(mockHTTP, "https://test.com", responses)
			}

			// Setup Discord expectations
			if tt.expectedPosts > 0 {
				mockDiscord.On("Send", "Progress updated!", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(tt.expectedPosts)
			}

			// Setup plugin
			plugin := ProgressPlugin{
				Url:           "https://test.com",
				Message:       "Progress updated!",
				DebounceDelay: tt.debounceDelay,
			}

			ctx := context.Background()
			pluginContext := PluginContext{
				Discord:    mockDiscord,
				Info:       log.New(os.Stdout, "", 0),
				Error:      log.New(os.Stderr, "", 0),
				Context:    &ctx,
				HTTPClient: mockHTTP,
			}

			// Initial offset - configurable per test case
			initialOffsetBuilder := NewOffsetBuilder()
			for _, progress := range tt.initialOffset {
				initialOffsetBuilder.AddProgress(progress.Title, progress.Value)
			}
			initialOffset := initialOffsetBuilder.Build()

			var currentOffset interface{} = initialOffset

			// Run through all responses
			for i := 0; i < len(tt.responses); i++ {
				offset, err := plugin.Check(currentOffset, pluginContext)
				assert.NoError(t, err)
				currentOffset = offset

				// Sleep after every response if specified
				if tt.sleepDuration > 0 {
					time.Sleep(tt.sleepDuration)
				}
			}

			// Verify mocks were called as expected
			mockHTTP.AssertExpectations(t)
			mockDiscord.AssertExpectations(t)

			// Validate final progress state
			finalOffset, ok := currentOffset.(ProgressOffset)
			assert.True(t, ok, "Expected ProgressOffset type")
			assert.Len(t, finalOffset.PublishedState, len(tt.expectedFinal), "Expected correct number of progress bars")
			
			for i, expected := range tt.expectedFinal {
				assert.Equal(t, expected.Title, finalOffset.PublishedState[i].Title, "Expected correct progress title")
				assert.Equal(t, expected.Value, finalOffset.PublishedState[i].Value, "Expected correct progress value")
			}

			// Validate clean debounce state (should always be clean at end)
			assert.False(t, finalOffset.IsDebouncing(), "Should have no pending changes after completion")
			assert.Nil(t, finalOffset.DebounceStart, "Should have no debounce start time after completion")
			
			// Always validate Discord messages when posts are expected
			validateDiscordMessages(t, mockDiscord.SentMessages, tt.expectedPosts, tt.initialOffset, tt.expectedFinal)
		})
	}
}

func TestProgressOffset_UnmarshalJSON(t *testing.T) {
	// Test new format
	newFormatJSON := `{
		"PublishedState": [{"Title": "Project A", "Link": "", "Value": 75}],
		"ObservedState": [{"Title": "Project A", "Link": "", "Value": 80}],
		"DebounceStart": "2023-10-15T14:30:00Z"
	}`
	
	var newOffset ProgressOffset
	err := json.Unmarshal([]byte(newFormatJSON), &newOffset)
	assert.NoError(t, err)
	assert.Len(t, newOffset.PublishedState, 1)
	assert.Equal(t, "Project A", newOffset.PublishedState[0].Title)
	assert.Equal(t, 75, newOffset.PublishedState[0].Value)
	assert.Len(t, newOffset.ObservedState, 1)
	assert.Equal(t, 80, newOffset.ObservedState[0].Value)
	assert.True(t, newOffset.IsDebouncing())
	assert.NotNil(t, newOffset.DebounceStart)
	
	// Test legacy format
	legacyFormatJSON := `[
		{"Title": "Project A", "Link": "", "Value": 50},
		{"Title": "Project B", "Link": "https://example.com", "Value": 80}
	]`
	
	var legacyOffset ProgressOffset
	err = json.Unmarshal([]byte(legacyFormatJSON), &legacyOffset)
	assert.NoError(t, err)
	assert.Len(t, legacyOffset.PublishedState, 2)
	assert.Equal(t, "Project A", legacyOffset.PublishedState[0].Title)
	assert.Equal(t, 50, legacyOffset.PublishedState[0].Value)
	assert.Equal(t, "Project B", legacyOffset.PublishedState[1].Title)
	assert.Equal(t, 80, legacyOffset.PublishedState[1].Value)
	assert.False(t, legacyOffset.IsDebouncing())
	assert.Nil(t, legacyOffset.DebounceStart)
}

func TestProgressPlugin_Validate(t *testing.T) {
	tests := []struct {
		name        string
		plugin      ProgressPlugin
		expectError string
	}{
		{
			name: "Valid plugin",
			plugin: ProgressPlugin{
				Url:           "https://example.com",
				Message:       "Progress updated",
				DebounceDelay: 5 * time.Second,
			},
			expectError: "",
		},
		{
			name: "Empty URL",
			plugin: ProgressPlugin{
				Message:       "Progress updated",
				DebounceDelay: 5 * time.Second,
			},
			expectError: "URL for progress updates must not be empty",
		},
		{
			name: "Empty message",
			plugin: ProgressPlugin{
				Url:           "https://example.com",
				DebounceDelay: 5 * time.Second,
			},
			expectError: "message for progress updates must not be empty",
		},
		{
			name: "Negative debounce delay",
			plugin: ProgressPlugin{
				Url:           "https://example.com",
				Message:       "Progress updated",
				DebounceDelay: -5 * time.Second,
			},
			expectError: "debounce delay must not be negative",
		},
		{
			name: "Zero debounce delay is valid",
			plugin: ProgressPlugin{
				Url:           "https://example.com",
				Message:       "Progress updated",
				DebounceDelay: 0,
			},
			expectError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plugin.Validate()
			if tt.expectError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			}
		})
	}
}
