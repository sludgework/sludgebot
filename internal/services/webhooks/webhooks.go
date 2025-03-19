package webhooks

import (
	"fmt"
	"strings"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/mjwhitta/log"
)

const (
	WebhookMsg = iota
	WebhookGitea
)

type HookType int

type WebhookHandler struct {
	DB         *DB
	HttpPrefix string
}

func (wh *WebhookHandler) FormURL(id string) string {
	return fmt.Sprintf("%s/%s", wh.HttpPrefix, id)
}

func (wh *WebhookHandler) HandleCreate(cmd string, msg chat1.MsgSummary) (url string, err error) {
	log.Infof("[webhooks] Creating webhook: [%+v]", msg)
	convID := msg.ConvID
	cmdTokens := strings.Split(cmd, " ")
	if len(cmdTokens) != 3 {
		return "", fmt.Errorf("[webhooks] Invalid arguments: [%+v]", cmdTokens)
	}

	name := cmdTokens[2]
	id, err := wh.DB.Create(name, convID)
	if err != nil {
		return "", fmt.Errorf("[webhooks] Failed to create webhook: %s", err)
	}

	return id, nil
}

func (wh *WebhookHandler) HandleList(_ string, msg chat1.MsgSummary) (webhooks []Webhook, err error) {
	return wh.DB.List(msg.ConvID)
}

func (h *WebhookHandler) HandleRemove(cmd string, msg chat1.MsgSummary) (err error) {
	convID := msg.ConvID
	cmdTokens := strings.Split(cmd, " ")
	if len(cmdTokens) != 3 {
		return fmt.Errorf("[webhooks] Invalid arguments: [%+v]", cmdTokens)
	}

	name := cmdTokens[2]
	if err := h.DB.Remove(name, convID); err != nil {
		return fmt.Errorf("[webhooks] Failed to remove webhook: %s", err)
	}

	return nil
}
