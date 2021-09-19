package sentryadam

import (
	"context"
	"strconv"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/mavolin/adam/pkg/bot"
	"github.com/mavolin/adam/pkg/errors"
	"github.com/mavolin/adam/pkg/plugin"
	"github.com/mavolin/disstate/v4/pkg/state"
)

type (
	Options struct {
		// Hub is the required hub used to power the wrapper.
		Hub *sentry.Hub

		// PerformanceSources are the names of the sources whose performance
		// shall be monitored.
		//
		// Set this to []string{} to disable performance monitoring.
		//
		// Performance monitoring may be further limited by the disabling
		// monitoring for certain commands globally.
		//
		// Default: []string{plugin.BuiltInSource}
		PerformanceSources []string
	}

	Wrapper struct {
		hub         *sentry.Hub
		perfSources map[string]struct{}
	}
)

// New creates a new *Wrapper from the given options.
//
// You must add all middlewares according to their documentation to the bot,
// in order for tracing to work.
func New(o Options) *Wrapper {
	var perfSources map[string]struct{}
	if o.PerformanceSources == nil {
		perfSources = map[string]struct{}{plugin.BuiltInSource: {}}
	} else {
		perfSources = make(map[string]struct{}, len(o.PerformanceSources))

		for _, src := range o.PerformanceSources {
			perfSources[src] = struct{}{}
		}
	}

	return &Wrapper{hub: o.Hub, perfSources: perfSources}
}

// PreRouteMiddleware is the first middleware to be added to the bot.
// It adds a *sentry.Hub to the *plugin.Context and starts a transaction.
//
// The bot's NoDefaultMiddlewares option must be enabled, and the default
// middleware must manually be added after the middleware returned by
// NewPreRouteMiddleware.
func (w *Wrapper) PreRouteMiddleware(next bot.CommandFunc) bot.CommandFunc {
	return func(s *state.State, ctx *plugin.Context) error {
		h := w.hub.Clone()

		if ctx.GuildID != 0 {
			shardID := s.GatewayFromGuildID(ctx.GuildID).Identifier.Shard.ShardID()
			h.Scope().SetTags(map[string]string{
				"guild_id": ctx.GuildID.String(),
				"shard_id": strconv.Itoa(shardID),
				"dm":       "false",
			})
		} else {
			h.Scope().SetTag("dm", "true")
		}

		h.Scope().SetTag("channel_id", ctx.ChannelID.String())
		h.Scope().SetExtra("message", ctx.Message)
		h.Scope().SetUser(sentry.User{
			ID:       ctx.Author.ID.String(),
			Username: ctx.Author.Username,
		})

		ctx.Set(sentry.HubContextKey, h)

		startParentSpan(ctx)
		defer finishParentSpan(ctx, w)

		startChildSpan(ctx, routeSpan)

		defer func() {
			if ctx.InvokedCommand != nil && w.monitorPerformance(ctx.InvokedCommand) {
				// ensure that the route span is finished, in case our next middleware
				// wasn't reached because a middleware in-between returned an error
				finishChildSpan(ctx, routeSpan)
			}
		}()

		err := next(s, ctx)
		setParentSpanStatus(ctx, err)
		return err
	}
}

// PostRouteMiddleware is the middleware that must be added immediately after
// the bot's default middlewares.
func (w *Wrapper) PostRouteMiddleware(next bot.CommandFunc) bot.CommandFunc {
	return func(s *state.State, ctx *plugin.Context) error {
		h := Hub(ctx)
		h.Scope().SetTags(map[string]string{
			"plugin_source": ctx.InvokedCommand.SourceName(),
			"command_id":    string(ctx.InvokedCommand.ID()),
		})
		h.Scope().SetTransaction(ctx.InvokedCommand.SourceName() + "/" + string(ctx.InvokedCommand.ID()[1:]))

		// don't even bother if we aren't monitoring perf
		if w.monitorPerformance(ctx.InvokedCommand) {
			finishChildSpan(ctx, routeSpan)
			startChildSpan(ctx, middlewaresSpan)

			// ensure that the middlewares span is finished, in case our next
			// middleware wasn't reached because a middleware in-between returned
			// an error
			defer finishChildSpan(ctx, middlewaresSpan)
		}

		return next(s, ctx)
	}
}

// PreInvokeMiddleware is the post middleware that must be added last.
func (w *Wrapper) PreInvokeMiddleware(next bot.CommandFunc) bot.CommandFunc {
	return func(s *state.State, ctx *plugin.Context) error {
		if w.monitorPerformance(ctx.InvokedCommand) {
			finishChildSpan(ctx, middlewaresSpan)

			startChildSpan(ctx, invokeSpan)
			defer finishChildSpan(ctx, invokeSpan)
		}

		return next(s, ctx)
	}
}

// =============================================================================
// Utils
// =====================================================================================

// monitorPerformance checks if the performance of the passed command shall be
// recorded.
func (w *Wrapper) monitorPerformance(cmd plugin.ResolvedCommand) bool {
	if _, ok := w.perfSources[cmd.SourceName()]; !ok {
		return false
	}

	_, noMonitoring := noPerformanceCmds[cmd.SourceName()+"/"+string(cmd.ID())]
	return !noMonitoring
}

// ================================ Span Getters ================================

type spanType string

const (
	parentSpan      spanType = "cmd"
	routeSpan       spanType = "route"
	middlewaresSpan spanType = "middlewares"
	invokeSpan      spanType = "invoke"
)

type (
	spanKey     string
	spanOnceKey string
)

func startParentSpan(ctx *plugin.Context) {
	spanCtx := sentry.SetHubOnContext(context.Background(), Hub(ctx))
	parent := sentry.StartSpan(spanCtx, string(parentSpan))

	ctx.Set(spanKey(parentSpan), parent)
}

func finishParentSpan(ctx *plugin.Context, w *Wrapper) {
	if ctx.InvokedCommand != nil && w.monitorPerformance(ctx.InvokedCommand) {
		ctx.Get(spanKey(parentSpan)).(*sentry.Span).Finish()
	}
}

func setParentSpanStatus(ctx *plugin.Context, err error) {
	parent := ctx.Get(spanKey(parentSpan)).(*sentry.Span)

	switch {
	case errors.Is(err, errors.Abort):
		parent.Status = sentry.SpanStatusAborted
	case errors.Is(err, bot.ErrUnknownCommand):
		parent.Status = sentry.SpanStatusNotFound
	case errors.As(err, new(*errors.InformationalError)):
		parent.Status = sentry.SpanStatusCanceled
	case errors.As(err, new(*errors.UserError)), errors.As(err, new(*errors.UserInfo)):
		parent.Status = sentry.SpanStatusFailedPrecondition
	case errors.As(err, new(*plugin.ArgumentError)):
		parent.Status = sentry.SpanStatusInvalidArgument
	case errors.As(err, new(*plugin.BotPermissionsError)):
		parent.Status = sentry.SpanStatusFailedPrecondition
	case errors.As(err, new(*plugin.ChannelTypeError)):
		parent.Status = sentry.SpanStatusFailedPrecondition
	case errors.As(err, new(*plugin.RestrictionError)):
		parent.Status = sentry.SpanStatusPermissionDenied
	case errors.As(err, new(*plugin.ThrottlingError)):
		parent.Status = sentry.SpanStatusResourceExhausted
	case errors.As(err, new(errors.Error)):
		parent.Status = sentry.SpanStatusUndefined
	case errors.As(err, new(*errors.InternalError)), err != nil:
		parent.Status = sentry.SpanStatusInternalError
	default:
		parent.Status = sentry.SpanStatusOK
	}
}

func startChildSpan(ctx *plugin.Context, childType spanType) {
	parent := ctx.Get(spanKey(parentSpan)).(*sentry.Span)

	child := parent.StartChild(string(childType))
	ctx.Set(spanKey(childType), child)
	ctx.Set(spanOnceKey(childType), new(sync.Once))
}

func finishChildSpan(ctx *plugin.Context, t spanType) {
	span := ctx.Get(spanKey(t)).(*sentry.Span)
	once := ctx.Get(spanOnceKey(t)).(*sync.Once)

	once.Do(span.Finish)
}
