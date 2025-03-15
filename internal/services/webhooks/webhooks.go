package webhooks

type HookType int

const (
	WEBHOOK_MSG = iota
	WEBHOOK_GITEA
)
