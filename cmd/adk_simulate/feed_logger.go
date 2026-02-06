package main

import (
	"encoding/json"

	"github.com/cpunion/sci-bot/pkg/feed"
	"github.com/cpunion/sci-bot/pkg/simulation"
)

type feedEventLogger struct {
	w *feed.Writer
}

func (l *feedEventLogger) LogEvent(ev simulation.EventLog) error {
	if l == nil || l.w == nil {
		return nil
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return l.w.AppendJSONLine(data)
}

func (l *feedEventLogger) Close() error {
	if l == nil || l.w == nil {
		return nil
	}
	return l.w.Close()
}
