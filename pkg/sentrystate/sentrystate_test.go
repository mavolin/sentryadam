package sentrystate

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"github.com/mavolin/adam/pkg/plugin"
	"github.com/mavolin/disstate/v4/pkg/event"
	"github.com/mavolin/disstate/v4/pkg/state"
	"github.com/stretchr/testify/assert"
)

func TestNewMiddleware(t *testing.T) {
	t.Parallel()
	t.Run("monitor perf", func(t *testing.T) {
		t.Parallel()

		_, s := state.NewMocker(t)

		m := NewMiddleware(HandlerMeta{
			Hub:                sentry.CurrentHub(),
			Operation:          "Test",
			MonitorPerformance: true,
		})

		done := make(chan struct{})

		s.AddHandler(func(_ *state.State, e *event.MessageCreate) {
			assert.NotNil(t, Hub(e.Base))
			assert.NotNil(t, Transaction(e.Base))

			done <- struct{}{}
		}, m)

		s.Call(&event.MessageCreate{Base: event.NewBase()})

		<-done
	})

	t.Run("don't monitor perf", func(t *testing.T) {
		t.Parallel()

		_, s := state.NewMocker(t)

		m := NewMiddleware(HandlerMeta{
			Hub:          sentry.CurrentHub(),
			PluginSource: plugin.BuiltInSource,
			PluginID:     "abc",
			Operation:    "Test",
		})

		done := make(chan struct{})

		s.AddHandler(func(_ *state.State, e *event.MessageCreate) {
			assert.NotNil(t, Hub(e.Base))
			assert.Nil(t, Transaction(e.Base))

			done <- struct{}{}
		}, m)

		s.Call(&event.MessageCreate{Base: event.NewBase()})

		<-done
	})
}
