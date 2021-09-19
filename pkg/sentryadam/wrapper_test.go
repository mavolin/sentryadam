package sentryadam

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/getsentry/sentry-go"
	"github.com/mavolin/adam/pkg/bot"
	"github.com/mavolin/adam/pkg/impl/command"
	"github.com/mavolin/adam/pkg/plugin"
	"github.com/mavolin/dismock/v3/pkg/dismock"
	"github.com/mavolin/disstate/v4/pkg/event"
	"github.com/mavolin/disstate/v4/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCommand struct {
	command.Meta

	hub  *sentry.Hub
	span *sentry.Span
}

func newTestCommand() *testCommand {
	return &testCommand{Meta: command.Meta{Name: "abc"}}
}

func (c *testCommand) Invoke(_ *state.State, ctx *plugin.Context) (interface{}, error) {
	c.hub = Hub(ctx)
	c.span = Transaction(ctx)

	return nil, nil
}

func TestWrapper(t *testing.T) {
	t.Run("no ignore", func(t *testing.T) {
		b := newTestBot(t, Options{Hub: sentry.CurrentHub()})

		cmd := newTestCommand()
		b.AddCommand(cmd)

		b.Route(event.NewBase(), &discord.Message{Content: "abc"}, nil)

		assert.NotNil(t, cmd.hub)
		assert.NotNil(t, cmd.span)
	})

	t.Run("source ignore", func(t *testing.T) {
		b := newTestBot(t, Options{
			Hub:                sentry.CurrentHub(),
			PerformanceSources: []string{},
		})

		cmd := newTestCommand()
		b.AddCommand(cmd)

		b.Route(event.NewBase(), &discord.Message{Content: "abc"}, nil)

		assert.NotNil(t, cmd.hub)
		assert.Nil(t, cmd.span)
	})

	t.Run("command ignore", func(t *testing.T) {
		b := newTestBot(t, Options{Hub: sentry.CurrentHub()})

		NoPerformanceMonitoring(plugin.BuiltInSource, ".abc")
		t.Cleanup(func() {
			noPerformanceCmds = make(map[string]struct{})
		})

		cmd := newTestCommand()
		b.AddCommand(cmd)

		b.Route(event.NewBase(), &discord.Message{Content: "abc"}, nil)

		assert.NotNil(t, cmd.hub)
		assert.Nil(t, cmd.span)
	})
}

func newTestBot(t *testing.T, o Options) *bot.Bot {
	t.Helper()

	m := dismock.New(t)

	m.MockAPI("BotURL", http.MethodGet, "gateway/bot", func(w http.ResponseWriter, _ *http.Request, t *testing.T) {
		err := json.NewEncoder(w).Encode(api.BotData{
			StartLimit: new(api.SessionStartLimit),
			Shards:     1,
		})
		require.NoError(t, err)
	})

	m.Me(discord.User{})
	m.Me(discord.User{})

	b, err := bot.New(bot.Options{
		Token:                "abc",
		HTTPClient:           m.HTTPClient(),
		NoDefaultMiddlewares: true,
	})
	require.NoError(t, err)

	wrapper := New(o)

	b.AddMiddleware(wrapper.PreRouteMiddleware)
	b.AddMiddleware(bot.CheckMessageType)
	b.AddMiddleware(bot.CheckHuman) // if Options.AllowBot is true
	b.AddMiddleware(bot.NewSettingsRetriever(bot.NewStaticSettingsProvider()))
	b.AddMiddleware(bot.CheckPrefix)
	b.AddMiddleware(bot.FindCommand)
	b.AddMiddleware(bot.CheckChannelTypes)
	b.AddMiddleware(bot.CheckBotPermissions)
	b.AddMiddleware(bot.NewThrottlerChecker(bot.DefaultThrottlerErrorCheck))
	b.AddMiddleware(wrapper.PostRouteMiddleware)

	b.AddPostMiddleware(bot.CheckRestrictions)
	b.AddPostMiddleware(bot.ParseArgs)
	b.AddPostMiddleware(wrapper.PreInvokeMiddleware)
	b.AddPostMiddleware(bot.InvokeCommand)

	return b
}
