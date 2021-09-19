// Package sentryadam provides utilities to use sentry with adam.
package sentryadam

import (
	"github.com/getsentry/sentry-go"
	"github.com/mavolin/adam/pkg/plugin"

	"github.com/mavolin/sentryadam/pkg/sentrystate"
)

var (
	HubKey  = sentrystate.HubKey
	SpanKey = spanKey(invokeSpan)
)

// Hub retrieves the *sentry.Hub from the passed *event.Base.
// In order for Hub to return with a non-nil *sentry.Hub, a hub must have
// previously been stored under the HubKey by a middleware, such as the
// three middlewares provided by sentryadam.
func Hub(ctx *plugin.Context) *sentry.Hub {
	return sentrystate.Hub(ctx.Base)
}

// Transaction retrieves the *sentry.Span from the passed *event.Base.
// In order for Transaction to return with a non-nil *sentry.Span, a spanType must
// have previously been stored under the SpanKey by a middleware, such as the
// three middlewares provided by sentryadam.
// Further, performance monitoring must be enabled for the command.
func Transaction(ctx *plugin.Context) *sentry.Span {
	if span := ctx.Get(SpanKey); span != nil {
		if span, ok := span.(*sentry.Span); ok && span != nil {
			return span
		}
	}

	return nil
}

var noPerformanceCmds = make(map[string]struct{})

// NoPerformanceMonitoring disables performance monitoring for the command
// with the given source name and id.
//
// Performance monitoring may be further limited depending on the settings
// given to the wrapper.
func NoPerformanceMonitoring(source string, id plugin.ID) {
	noPerformanceCmds[source+"/"+string(id)] = struct{}{}
}
