package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aprzybys/sludgebot/internal/services/webhooks"
	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
)

type Handler struct {
	*base.DebugOutput

	stats   *base.StatsRegistry
	kbc     *kbchat.API
	httpSrv *HTTPSrv
	wh      *webhooks.WebhookHandler
}

var _ base.Handler = (*Handler)(nil)

var errNotAllowed = errors.New("must be at least a writer to administer webhooks")

func NewHandler(stats *base.StatsRegistry, kbc *kbchat.API, debugConfig *base.ChatDebugOutputConfig,
	httpSrv *HTTPSrv, wh *webhooks.WebhookHandler) *Handler {
	return &Handler{
		DebugOutput: base.NewDebugOutput("Handler", debugConfig),
		stats:       stats.SetPrefix("Handler"),
		kbc:         kbc,
		wh:          wh,
		httpSrv:     httpSrv,
	}
}

func (h *Handler) CheckAllowed(msg chat1.MsgSummary) error {
	ok, err := base.IsAtLeastWriter(h.kbc, msg.Sender.Username, msg.Channel)
	if err != nil {
		return fmt.Errorf("handleCreate: failed to check role: %s", err)
	}
	if !ok {
		return errNotAllowed
	}
	return nil
}

func (h *Handler) HandleNewConv(conv chat1.ConvSummary) error {
	welcomeMsg := "I can create generic webhooks into Keybase! Try `!webhook create` to get started."
	return base.HandleNewTeam(h.stats, h.DebugOutput, h.kbc, conv, welcomeMsg)
}

func (h *Handler) HandleCommand(msg chat1.MsgSummary) error {
	if msg.Content.Text == nil {
		return nil
	}

	cmd := strings.TrimSpace(msg.Content.Text.Body)
	switch {
	case strings.HasPrefix(cmd, "!msghook create"):
		id, err := h.wh.HandleCreate(cmd, msg)
		if err != nil {
			return err
		}
		_, err = h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", id)
		return err
	case strings.HasPrefix(cmd, "!msghook list"):
		hooks, err := h.wh.HandleList(cmd, msg)
		if err != nil {
			return err
		} else if len(hooks) == 0 {
			h.ChatEcho(msg.ConvID, "No hooks in this conversation")
			return nil
		}

		var body string
		for _, hook := range hooks {
			body += fmt.Sprintf("%s, %s\n", hook.Name, h.wh.FormURL(hook.ID))
		}
		h.kbc.SendMessageByTlfName(msg.Sender.Username, "%s", body)

	case strings.HasPrefix(cmd, "!msghook remove"):
		return h.wh.HandleRemove(cmd, msg)
	}
	return nil
}
