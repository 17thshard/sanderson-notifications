package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockProgressDiffRenderer struct {
	mock.Mock
}

func (r *MockProgressDiffRenderer) Render(progressBars []ProgressDiff) string {
	args := r.Called(progressBars)
	return args.String(0)
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

func TestProgress_Validate(t *testing.T) {
	tests := []struct {
		name        string
		plugin      ProgressPlugin
		expectError string
	}{
		{
			name: "Valid config",
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

func TestProgress_UnmarshalOffset(t *testing.T) {
	// Test new format
	newFormatJSON := `{
		"Published": [{"Title": "Project A", "Link": "", "Value": 75}],
		"Observed": [{"Title": "Project A", "Link": "", "Value": 80}],
		"DebounceStart": "2023-10-15T14:30:00Z"
	}`

	var newOffset ProgressOffset
	err := json.Unmarshal([]byte(newFormatJSON), &newOffset)
	assert.NoError(t, err)
	expectedTime := time.Date(2023, 10, 15, 14, 30, 0, 0, time.UTC)
	assert.Equal(t, ProgressOffset{
		Published:     []Progress{{Title: "Project A", Value: 75}},
		Observed:      []Progress{{Title: "Project A", Value: 80}},
		DebounceStart: &expectedTime,
	}, newOffset)

	// Test legacy format
	legacyFormatJSON := `[
		{"Title": "Project A", "Link": "", "Value": 50},
		{"Title": "Project B", "Link": "https://example.com", "Value": 80}
	]`

	var legacyOffset ProgressOffset
	err = json.Unmarshal([]byte(legacyFormatJSON), &legacyOffset)
	assert.NoError(t, err)
	assert.Equal(t, ProgressOffset{
		Published:     []Progress{{Title: "Project A", Value: 50}, {Title: "Project B", Link: "https://example.com", Value: 80}},
		Observed:      []Progress{{Title: "Project A", Value: 50}, {Title: "Project B", Link: "https://example.com", Value: 80}},
		DebounceStart: nil,
	}, legacyOffset)
	assert.Len(t, legacyOffset.Published, 2)
	assert.Equal(t, "Project A", legacyOffset.Published[0].Title)
	assert.Equal(t, 50, legacyOffset.Published[0].Value)
	assert.Equal(t, "Project B", legacyOffset.Published[1].Title)
	assert.Equal(t, 80, legacyOffset.Published[1].Value)
	assert.Nil(t, legacyOffset.DebounceStart)
}

func TestProgress_Diff(t *testing.T) {
	scenarios := []struct {
		name     string
		old      []Progress
		new      []Progress
		expected []ProgressDiff
	}{
		{
			name: "No changes - identical progress bars",
			old: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 75},
			},
			new: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 75},
			},
			expected: nil,
		},
		{
			name: "No changes - different order",
			old: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 75},
			},
			new: []Progress{
				{Title: "Project B", Value: 75},
				{Title: "Project A", Value: 50},
			},
			expected: nil,
		},
		{
			name: "No changes - removed bar",
			old: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 75},
			},
			new: []Progress{
				{Title: "Project A", Value: 50},
			},
			expected: nil,
		},
		{
			name: "New bar",
			old: []Progress{
				{Title: "Project A", Value: 50},
			},
			new: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 75},
			},
			expected: []ProgressDiff{
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project B",
					OldValue: 0,
					Value:    75,
					New:      true,
				},
			},
		},
		{
			name: "Changed bar",
			old: []Progress{
				{Title: "Project A", Value: 50},
			},
			new: []Progress{
				{Title: "Project A", Value: 75},
			},
			expected: []ProgressDiff{
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    75,
					New:      false,
				},
			},
		},
		{
			name: "Mixed changes - uses order of new value",
			old: []Progress{
				{Title: "Project A", Value: 50},
				{Title: "Project B", Value: 50},
				{Title: "Project C", Value: 50},
			},
			new: []Progress{
				{Title: "Project C", Value: 75},
				{Title: "Project A", Value: 50},
				{Title: "Project D", Value: 75},
			},
			expected: []ProgressDiff{
				{
					Title:    "Project C",
					OldValue: 50,
					Value:    75,
					New:      false,
				},
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project D",
					OldValue: 0,
					Value:    75,
					New:      true,
				},
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			assert.Equal(t, s.expected, diff(s.old, s.new))
		})
	}
}

func TestProgressRenderer_Render(t *testing.T) {
	scenarios := []struct {
		name     string
		changes  []ProgressDiff
		expected string
	}{
		{
			name: "New bar",
			changes: []ProgressDiff{
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project B",
					OldValue: 0,
					Value:    75,
					New:      true,
				},
			},
			expected: "**Project A**\n`████████████████████░░░░░░░░░░░░░░░░░░░░  50%`\n\n**[New] Project B**\n`██████████████████████████████░░░░░░░░░░  75%`",
		},
		{
			name: "Changed bar",
			changes: []ProgressDiff{
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    75,
					New:      false,
				},
			},
			expected: "**[Changed] Project A (50% → 75%)**\n`██████████████████████████████░░░░░░░░░░  75%`",
		},
		{
			name: "Linked bar",
			changes: []ProgressDiff{
				{
					Title:    "Project A",
					Link:     "https://example.com",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
			},
			expected: "**[Project A](https://example.com)**\n`████████████████████░░░░░░░░░░░░░░░░░░░░  50%`",
		},
		{
			name: "Mixed changes - uses order of new value",
			changes: []ProgressDiff{
				{
					Title:    "Project C",
					OldValue: 50,
					Value:    75,
					New:      false,
				},
				{
					Title:    "Project A",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project D",
					OldValue: 0,
					Value:    75,
					New:      true,
				},
			},
			expected: "**[Changed] Project C (50% → 75%)**\n`██████████████████████████████░░░░░░░░░░  75%`\n\n**Project A**\n`████████████████████░░░░░░░░░░░░░░░░░░░░  50%`\n\n**[New] Project D**\n`██████████████████████████████░░░░░░░░░░  75%`",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			renderer := progressRenderer{}
			assert.Equal(t, s.expected, renderer.Render(s.changes))
		})
	}
}

// ProgressCheckScenario for creating test scenarios with fluent API
type ProgressCheckScenario struct {
	name                  string
	debounceDelay         time.Duration
	responses             []string
	sleepDuration         time.Duration
	expectedReportedDiffs [][]ProgressDiff
	expectedFinal         []Progress
	initialOffset         []Progress
	description           string
}

func scenario(name string) *ProgressCheckScenario {
	return &ProgressCheckScenario{name: name}
}

func (s *ProgressCheckScenario) Debounce(delay time.Duration) *ProgressCheckScenario {
	s.debounceDelay = delay
	return s
}

func (s *ProgressCheckScenario) NoDebounce() *ProgressCheckScenario {
	s.debounceDelay = 0
	return s
}

func (s *ProgressCheckScenario) ResponseSpecs(specs ...[]Progress) *ProgressCheckScenario {
	s.responses = make([]string, len(specs))
	for i, spec := range specs {
		html := `<html><body>`
		for _, bar := range spec {
			html += fmt.Sprintf(
				`
				<div class="progress-item-template-123">
					<div class="progress-title-template-456">%s</div>
					<div class="progress-percent-template-789">%d%%</div>
				</div>
				`,
				bar.Title,
				bar.Value,
			)
		}
		html += `</body></html>`
		s.responses[i] = html
	}
	return s
}

func (s *ProgressCheckScenario) Sleep(duration time.Duration) *ProgressCheckScenario {
	s.sleepDuration = duration
	return s
}

func (s *ProgressCheckScenario) ExpectReports(diffs ...[]ProgressDiff) *ProgressCheckScenario {
	s.expectedReportedDiffs = diffs
	return s
}

func (s *ProgressCheckScenario) ExpectNoReports() *ProgressCheckScenario {
	s.expectedReportedDiffs = nil
	return s
}

func (s *ProgressCheckScenario) InitialProgress(title string, value int) *ProgressCheckScenario {
	s.initialOffset = []Progress{{Title: title, Value: value}}
	return s
}

func (s *ProgressCheckScenario) InitialProgressMultiple(progress ...Progress) *ProgressCheckScenario {
	s.initialOffset = progress
	return s
}

func (s *ProgressCheckScenario) FinalProgress(title string, value int) *ProgressCheckScenario {
	s.expectedFinal = []Progress{{Title: title, Value: value}}
	return s
}

func (s *ProgressCheckScenario) FinalProgressMultiple(progress ...Progress) *ProgressCheckScenario {
	s.expectedFinal = progress
	return s
}

func (s *ProgressCheckScenario) Description(desc string) *ProgressCheckScenario {
	s.description = desc
	return s
}

func TestProgress_Check(t *testing.T) {
	scenarios := []*ProgressCheckScenario{
		scenario("Zero debounce").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project A", "", 75}}, // No debounce, post immediately
			).
			ExpectReports([]ProgressDiff{{
				Title:    "Project A",
				Link:     "",
				OldValue: 50,
				Value:    75,
				New:      false,
			}}).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("Should work like legacy mode"),

		scenario("Multiple rapid changes").
			Debounce(10*time.Millisecond).
			ResponseSpecs(
				[]Progress{{"Project A", "", 75}}, // Starts debounce
				[]Progress{{"Project A", "", 85}}, // Resets debounce
				[]Progress{{"Project A", "", 75}}, // Resets debounce
				[]Progress{{"Project A", "", 75}}, // Continues debounce time
				[]Progress{{"Project A", "", 75}}, // Continues debounce time
				[]Progress{{"Project A", "", 75}}, // Debounce time elapses (post)
			).
			Sleep(4*time.Millisecond).
			ExpectReports([]ProgressDiff{{
				Title:    "Project A",
				Link:     "",
				OldValue: 50,
				Value:    75,
				New:      false,
			}}).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("75% -> 85% -> 75% -> 75% (stable, posts final)"),

		scenario("Very short debounce").
			Debounce(1*time.Millisecond).
			ResponseSpecs(
				[]Progress{{"Project A", "", 75}}, // Starts debounce
				[]Progress{{"Project A", "", 75}}, // Debounce elapsed, post change
			).
			Sleep(2*time.Millisecond).
			ExpectReports([]ProgressDiff{{
				Title:    "Project A",
				Link:     "",
				OldValue: 50,
				Value:    75,
				New:      false,
			}}).
			InitialProgress("Project A", 50).
			FinalProgress("Project A", 75).
			Description("Timing edge case"),

		scenario("Revert to original").
			Debounce(10*time.Millisecond).
			ResponseSpecs(
				[]Progress{{"Project A", "", 75}, {"Project B", "", 30}}, // Starts debounce
				[]Progress{{"Project A", "", 50}, {"Project B", "", 30}}, // Debounce elapsed, no change between posted so no new post
			).
			Sleep(15*time.Millisecond).
			ExpectNoReports().
			InitialProgressMultiple(
				Progress{"Project A", "", 50},
				Progress{"Project B", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project A", "", 50},
				Progress{"Project B", "", 30},
			).
			Description("No post when back to original"),

		scenario("Multiple bars with debounce").
			Debounce(10*time.Millisecond).
			ResponseSpecs(
				[]Progress{{"Project A", "", 75}, {"Project B", "", 30}}, // Starts debounce
				[]Progress{{"Project A", "", 75}, {"Project B", "", 60}}, // Resets debounce (change detected from last observed state)
				[]Progress{{"Project A", "", 90}, {"Project B", "", 60}}, // Resets debounce (change detected from last observed state)
				[]Progress{{"Project A", "", 90}, {"Project B", "", 60}}, // Continues debounce
				[]Progress{{"Project A", "", 90}, {"Project B", "", 60}}, // Debounce elapsed, post change
			).
			Sleep(9*time.Millisecond).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project A",
					Link:     "",
					OldValue: 50,
					Value:    90,
					New:      false,
				},
				{
					Title:    "Project B",
					Link:     "",
					OldValue: 30,
					Value:    60,
					New:      false,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project A", "", 50},
				Progress{"Project B", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project A", "", 90},
				Progress{"Project B", "", 60},
			).
			Description("Bar A changes → Bar B changes → Bar A changes again → stable"),

		scenario("New progress bar appears").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 1", "", 50}, {"Project 2", "", 30}, {"Project 3", "", 80}}, // No debounce, post immediately
			).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project 1",
					Link:     "",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project 2",
					Link:     "",
					OldValue: 30,
					Value:    30,
					New:      false,
				},
				{
					Title:    "Project 3",
					Link:     "",
					OldValue: 0,
					Value:    80,
					New:      true,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
				Progress{"Project 3", "", 80},
			).
			Description("New bar detected as change"),

		scenario("Multiple bars mixed changes").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 1", "", 75}, {"Project 2", "", 60}}, // No debounce, post immediately
			).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project 1",
					Link:     "",
					OldValue: 50,
					Value:    75,
					New:      false,
				},
				{
					Title:    "Project 2",
					Link:     "",
					OldValue: 30,
					Value:    60,
					New:      false,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 75},
				Progress{"Project 2", "", 60},
			).
			Description("Both bars change values"),

		scenario("Same bars different order").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 2", "", 30}, {"Project 1", "", 50}}, // No debounce, post immediately
			).
			ExpectNoReports().
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			Description("Order independence - no changes"),

		scenario("Progress bar removed").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 1", "", 50}, {"Project 2", "", 30}}, // No debounce, post immediately
				[]Progress{{"Project 1", "", 50}},                        // No debounce, does not post because removing a bar does not cause a post if there are no other changes
			).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project 1",
					Link:     "",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project 2",
					Link:     "",
					OldValue: 0,
					Value:    30,
					New:      true,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			Description("Removal preserves old state in case a bar comes back"),

		scenario("Remove then add different bar").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 1", "", 50}},                        // No debounce, but also no post
				[]Progress{{"Project 1", "", 50}, {"Project 3", "", 80}}, // No debounce, post immediately
			).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project 1",
					Link:     "",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project 3",
					Link:     "",
					OldValue: 0,
					Value:    80,
					New:      true,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 3", "", 80},
			).
			Description("Remove bar (no update) then add different bar (updates state)"),

		scenario("New, changed, and removed bars mixed").
			NoDebounce().
			ResponseSpecs(
				[]Progress{{"Project 1", "", 50}, {"Project 3", "", 40}, {"Project 4", "", 80}}, // No debounce, post immediately
			).
			ExpectReports([]ProgressDiff{
				{
					Title:    "Project 1",
					Link:     "",
					OldValue: 50,
					Value:    50,
					New:      false,
				},
				{
					Title:    "Project 3",
					Link:     "",
					OldValue: 0,
					Value:    40,
					New:      true,
				},
				{
					Title:    "Project 4",
					Link:     "",
					OldValue: 20,
					Value:    80,
					New:      false,
				},
			}).
			InitialProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 2", "", 30},
				Progress{"Project 4", "", 20},
			).
			FinalProgressMultiple(
				Progress{"Project 1", "", 50},
				Progress{"Project 3", "", 40},
				Progress{"Project 4", "", 80},
			).
			Description("Remove bar (no update) then add different bar (updates state)"),
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			// Setup mocks
			mockHTTP := &MockHTTPClient{}
			mockDiscord := &MockDiscordClient{}
			mockDiffRenderer := &MockProgressDiffRenderer{}
			mockClock := &MockClock{}

			// Setup HTTP sequence if we have responses
			if len(s.responses) > 0 {
				responses := s.responses
				setupHTTPSequence(mockHTTP, "https://test.com", responses)
			}

			// Setup Discord expectations
			if s.expectedReportedDiffs != nil {
				mockDiscord.On("Send", "Progress updated!", mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(len(s.expectedReportedDiffs))
			}

			// Setup renderer expectations
			for _, expected := range s.expectedReportedDiffs {
				mockDiffRenderer.On("Render", expected).Return("progress bars").Once()
			}

			// Setup plugin
			plugin := ProgressPlugin{
				Url:           "https://test.com",
				Message:       "Progress updated!",
				DebounceDelay: s.debounceDelay,

				clock:    mockClock,
				renderer: mockDiffRenderer,
			}

			ctx := context.Background()
			pluginContext := PluginContext{
				Discord: mockDiscord,
				Info:    log.New(os.Stdout, "", 0),
				Error:   log.New(os.Stderr, "", 0),
				Context: &ctx,
				HTTP:    mockHTTP,
			}

			// Initial offset - configurable per test case
			initialOffset := ProgressOffset{
				Published:     s.initialOffset,
				Observed:      s.initialOffset,
				DebounceStart: nil,
			}

			var currentOffset interface{} = initialOffset

			// Run through all responses
			for i := 0; i < len(s.responses); i++ {
				offset, err := plugin.Check(currentOffset, pluginContext)
				assert.NoError(t, err)
				currentOffset = offset

				// Sleep after every response if specified
				if s.sleepDuration > 0 {
					mockClock.Advance(s.sleepDuration)
				}
			}

			// Verify mocks were called as expected
			mockHTTP.AssertExpectations(t)
			mockDiscord.AssertExpectations(t)
			mockDiffRenderer.AssertExpectations(t)

			// Validate final progress state
			finalOffset, ok := currentOffset.(ProgressOffset)
			assert.True(t, ok, "Expected ProgressOffset type")

			assert.Equal(t, s.expectedFinal, finalOffset.Published)

			// Validate clean debounce state (should always be clean at end)
			assert.Nil(t, finalOffset.DebounceStart, "Should have no debounce start time after completion")

			// Always validate Discord messages
			assert.Len(t, mockDiscord.SentMessages, len(s.expectedReportedDiffs), "Should send expected number of messages")

			for index, message := range mockDiscord.SentMessages {
				assert.Equal(t, DiscordMessage{
					Text:   "Progress updated!",
					Name:   "Progress Updates",
					Avatar: "dragonsteel",
					Embed: map[string]interface{}{
						"description": "progress bars", // from mock
						"footer": map[string]interface{}{
							"text": "See https://test.com for more",
						},
					},
				}, message, fmt.Sprintf("Message #%d had unexpected value", index))
			}
		})
	}
}
