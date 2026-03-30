package translator

import "time"

type TranslatorTraceEvent struct {
	Time    time.Time
	Engine  string
	Stage   string
	Message string
	Fields  map[string]any
}

type TraceSinkSetter interface {
	SetTraceSink(func(TranslatorTraceEvent))
}
