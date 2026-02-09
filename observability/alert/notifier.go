package alert

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/code19m/errx"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/discord"
	"github.com/nikoksr/notify/service/telegram"
)

type notifier interface {
	notify(ctx context.Context, e errorInfo) error
}

func newNotifier(cfg Config) (notifier, error) {
	env := os.Getenv("ENVIRONMENT")

	switch cfg.Provider {
	case providerDiscord:
		return newDiscordNotifier(cfg.DiscordBotToken, cfg.DiscordChannelIDs, env)
	case providerTelegram:
		return newTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatIDs, env)
	default:
		return nil, errx.New("invalid alert provider: " + cfg.Provider)
	}
}

// --- Message formatting ---

// bodyFormats defines format strings for building notification messages.
type bodyFormats struct {
	escape       func(string) string
	fieldFmt     string // format for header fields: takes label and value
	detailHeader string // separator before details section
	detailFmt    string // format for each detail entry: takes key and value
	freqFmt      string // format for frequency line: takes count and minutes
}

func getDiscordFormats() bodyFormats {
	return bodyFormats{
		escape:       escapeMarkdown,
		fieldFmt:     "**%s:** %s\n",
		detailHeader: "\n**📋 _Additional details_**\n",
		detailFmt:    "_%s_: ```%s```",
		freqFmt:      "\n**📊 Frequency:** %d in last %d minutes",
	}
}

func getTelegramFormats() bodyFormats {
	return bodyFormats{
		escape:       escapeHTML,
		fieldFmt:     "<b>%s:</b> %s\n",
		detailHeader: "\n<b>📋 <i>Additional details</i></b>\n",
		detailFmt:    "<i>%s</i>: <code>%s</code>\n",
		freqFmt:      "\n<b>📊 Frequency:</b> %d in last %d minutes",
	}
}

func buildBody(e errorInfo, environment string, f bodyFormats) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf(f.fieldFmt, "🔍 Environment", f.escape(environment)))
	buf.WriteString(fmt.Sprintf(f.fieldFmt, "🛠️ Service", f.escape(e.service)))
	buf.WriteString(fmt.Sprintf(f.fieldFmt, "🔄 Operation", f.escape(e.operation)))
	buf.WriteString(fmt.Sprintf(f.fieldFmt, "🏷️ Code", f.escape(e.code)))
	buf.WriteString(fmt.Sprintf(f.fieldFmt, "💬 Message", f.escape(e.message)))

	buf.WriteString(f.detailHeader)

	for k, v := range e.details {
		if v != "" {
			if len(v) > 1000 {
				v = v[:1000] + "..."
			}
			buf.WriteString(fmt.Sprintf(f.detailFmt, f.escape(k), v))
		}
	}

	if e.frequency > 0 {
		buf.WriteString(fmt.Sprintf(f.freqFmt, e.frequency, e.frequencyMinutes))
	}

	return buf.String()
}

// --- Discord ---

type discordNotifier struct {
	n           notify.Notifier
	environment string
}

func newDiscordNotifier(token string, channelIDs []string, environment string) (*discordNotifier, error) {
	d := discord.New()
	if err := d.AuthenticateWithBotToken(token); err != nil {
		return nil, errx.Wrap(err)
	}
	d.AddReceivers(channelIDs...)

	n := notify.New()
	n.UseServices(d)

	return &discordNotifier{
		n:           n,
		environment: environment,
	}, nil
}

func (dn *discordNotifier) notify(ctx context.Context, e errorInfo) error {
	title := "**❗ Error Alert**\n"
	body := buildBody(e, dn.environment, getDiscordFormats())

	if err := dn.n.Send(ctx, title, body); err != nil {
		return errx.Wrap(err)
	}

	return nil
}

// --- Telegram ---

type telegramNotifier struct {
	n           notify.Notifier
	environment string
}

func newTelegramNotifier(token string, chatIDs []int64, environment string) (*telegramNotifier, error) {
	tg, err := telegram.New(token)
	if err != nil {
		return nil, errx.Wrap(err)
	}
	tg.AddReceivers(chatIDs...)

	n := notify.New()
	n.UseServices(tg)

	return &telegramNotifier{
		n:           n,
		environment: environment,
	}, nil
}

func (tn *telegramNotifier) notify(ctx context.Context, e errorInfo) error {
	title := "<b>❗ Error Alert</b>\n"
	body := buildBody(e, tn.environment, getTelegramFormats())

	if err := tn.n.Send(ctx, title, body); err != nil {
		return errx.Wrap(err)
	}

	return nil
}

// --- Escape utilities ---

func escapeMarkdown(in string) string {
	replacer := strings.NewReplacer(
		"*", "\\*",
		"_", "\\_",
		"`", "\\`",
		"~", "\\~",
		"|", "\\|",
	)
	return replacer.Replace(replaceNewlines(in))
}

func escapeHTML(in string) string {
	return html.EscapeString(replaceNewlines(in))
}

func replaceNewlines(in string) string {
	return strings.ReplaceAll(in, "\n", "\\n")
}
