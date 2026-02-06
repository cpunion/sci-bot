package simulation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	ailibmodel "github.com/cpunion/ailib/adk/model"
	"github.com/cpunion/sci-bot/pkg/publication"
	"github.com/cpunion/sci-bot/pkg/types"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

type memoryLogger struct {
	events []EventLog
}

func (l *memoryLogger) LogEvent(ev EventLog) error {
	l.events = append(l.events, ev)
	return nil
}

func (l *memoryLogger) Close() error { return nil }

func TestADKScheduler_LogsTokenUsage(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()

	mock := ailibmodel.NewMockLLM(&adkmodel.LLMResponse{
		Content: &genai.Content{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "hello"},
			},
		},
		UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     11,
			CandidatesTokenCount: 22,
			TotalTokenCount:      33,
		},
	})

	logger := &memoryLogger{}
	sched := NewADKScheduler(ADKSchedulerConfig{
		DataPath:        tempDir,
		Model:           mock,
		Logger:          logger,
		SimStep:         time.Hour,
		StartTime:       time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		AgentsPerTick:   1,
		CheckpointEvery: 1000,
	})
	sched.SetJournal(publication.NewJournal("J", filepath.Join(tempDir, "journal")))
	sched.SetForum(publication.NewForum("F", filepath.Join(tempDir, "forum")))

	persona := &types.Persona{
		ID:            "agent-1",
		Name:          "Tester",
		Role:          types.RoleExplorer,
		ThinkingStyle: types.StyleDivergent,
		Creativity:    0.5,
		RiskTolerance: 0.5,
		Rigor:         0.5,
		Sociability:   0.5,
		Influence:     0.5,
	}

	ctx := context.Background()
	if err := sched.AddAgent(ctx, persona); err != nil {
		t.Fatalf("AddAgent: %v", err)
	}
	if err := sched.RunTick(ctx); err != nil {
		t.Fatalf("RunTick: %v", err)
	}

	if len(logger.events) != 1 {
		t.Fatalf("expected 1 log event, got %d", len(logger.events))
	}
	ev := logger.events[0]
	if ev.ModelName != "mock-llm" {
		t.Fatalf("expected model_name=mock-llm, got %q", ev.ModelName)
	}
	if ev.UsageEvents != 1 {
		t.Fatalf("expected usage_events=1, got %d", ev.UsageEvents)
	}
	if ev.PromptTokens != 11 || ev.CandidatesTokens != 22 || ev.TotalTokens != 33 {
		t.Fatalf("expected tokens 11/22/33, got prompt=%d candidates=%d total=%d", ev.PromptTokens, ev.CandidatesTokens, ev.TotalTokens)
	}
}

