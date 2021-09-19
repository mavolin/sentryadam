// Package sentrystate provides utilities to use sentry with disstate.
package sentrystate

import (
	"context"
	"reflect"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/mavolin/adam/pkg/plugin"
	"github.com/mavolin/disstate/v4/pkg/event"
	"github.com/mavolin/disstate/v4/pkg/state"
)

type spanKey struct{}

var (
	HubKey  = sentry.HubContextKey
	SpanKey = new(spanKey)
)

// Hub retrieves the *sentry.Hub from the passed *event.Base.
// In order for Hub to return with a non-nil *sentry.Hub, a hub must have
// previously been stored under the HubKey by a middleware such as the one
// returned by NewMiddleware.
func Hub(b *event.Base) *sentry.Hub {
	if hub := b.Get(HubKey); hub != nil {
		if hub, ok := hub.(*sentry.Hub); ok && hub != nil {
			return hub
		}
	}

	return nil
}

// Transaction retrieves the *sentry.Span from the passed *event.Base.
// In order for Transaction to return with a non-nil *sentry.Span, a span must
// have previously been stored under the SpanKey by a middleware such as the
// one returned by NewMiddleware.
func Transaction(b *event.Base) *sentry.Span {
	if span := b.Get(SpanKey); span != nil {
		if span, ok := span.(*sentry.Span); ok && span != nil {
			return span
		}
	}

	return nil
}

// HandlerMeta holds meta data about a handler.
type HandlerMeta struct {
	// Hub is the required *sentry.Hub assigned to the handler.
	Hub *sentry.Hub
	// PluginSource is the optional name of the plugin source this handler
	// belongs to.
	PluginSource string
	// PluginID is the optional id of the plugin this handler belongs to.
	PluginID plugin.ID
	// Operation is the name of the operation this handler performs.
	Operation string
	// MonitorPerformance specifies whether to enable performance monitoring
	// for the handler.
	// Before returning, `Transaction(e.Base).Finsish()` must be called.
	MonitorPerformance bool
}

// NewMiddleware creates a new middleware that attaches a *sentry.Hub to the
// event's base.
func NewMiddleware(m HandlerMeta) func(*state.State, interface{}) {
	var transactionBuilder strings.Builder
	transactionBuilder.Grow(
		len(m.PluginSource) + len("/") + len(m.PluginID) + len("/") + len(m.Operation),
	)

	if m.PluginSource != "" {
		transactionBuilder.WriteString(m.PluginSource)
		transactionBuilder.WriteRune('/')
	}

	if m.PluginID != "" {
		transactionBuilder.WriteString(string(m.PluginID))
		transactionBuilder.WriteRune('/')
	}

	transactionBuilder.WriteString(m.Operation)

	transactionName := transactionBuilder.String()

	return func(_ *state.State, e interface{}) {
		re := reflect.ValueOf(e).Elem()
		b := re.FieldByName("Base").Interface().(*event.Base)

		h := m.Hub.Clone()

		h.Scope().SetTransaction(transactionName)
		h.Scope().SetExtra("event", e)

		b.Set(HubKey, h)

		if m.MonitorPerformance {
			ctx := sentry.SetHubOnContext(context.Background(), h)
			span := sentry.StartSpan(ctx, m.Operation)
			b.Set(SpanKey, span)
		}
	}
}
