package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
	"github.com/mjwhitta/log"
)

const (
	sqlCreate string = `` +
		`INSERT INTO hooks (id, name, conv_id, hook_type) VALUES ` +
		`(?, ?, ?, ?)`
	sqlDeleteHook string = `` +
		`DELETE FROM hooks WHERE conv_id = ? AND name = ?`
	sqlGetHook string = `` +
		`SELECT conv_id, name, hook_type FROM hooks WHERE id = ?`
	sqlGetHooks string = `` +
		`SELECT id, name, hook_type FROM hooks WHERE conv_id = ?`
)

type DB struct {
	*base.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{
		DB: base.NewDB(db),
	}
}

func (d *DB) makeID(name string, convID chat1.ConvIDStr) (string, error) {
	secret, err := base.RandBytes(16)
	if err != nil {
		return "", err
	}
	cdat, err := hex.DecodeString(string(convID))
	if err != nil {
		return "", err
	}
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write(cdat)
	_, _ = h.Write([]byte(name))
	return base.URLEncoder().EncodeToString(h.Sum(nil)[:20]), nil
}

func (d *DB) Create(name string, convID chat1.ConvIDStr) (string, error) {
	var hookType int = WebhookMsg
	id, err := d.makeID(name, convID)
	if err != nil {
		return "", err
	}

	log.Infof("Creating webhook [%s: %+v]", name, convID)
	err = d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			sqlCreate,
			id,
			name,
			convID,
			hookType,
		)
		if err != nil {
			log.Errf("Failed to create webhook: %+v", err)
			return err
		}
		return nil
	})
	return id, err
}

func (d *DB) GetHook(id string) (res Webhook, err error) {
	log.Infof("Retrieving webhook [%s]", id)
	row := d.DB.QueryRow(sqlGetHook, id)
	if err := row.Scan(&res.ConvID, &res.Name, &res.HookType); err != nil {
		log.Errf("Failed to retrieve webhook: [%+v]: %+v", res, err)
		return res, err
	}
	return res, nil
}

type Webhook struct {
	ID       string
	ConvID   chat1.ConvIDStr
	HookType HookType
	Name     string
}

func (d *DB) List(convID chat1.ConvIDStr) (res []Webhook, err error) {
	log.Infof("Listing webhooks [%+v]", convID)
	rows, err := d.DB.Query(sqlGetHooks, convID)
	if err != nil {
		log.Errf("Failed to list webhooks: [%+v]: %+v", convID, err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hook Webhook
		hook.ConvID = convID
		if err := rows.Scan(&hook.ID, &hook.Name, &hook.HookType); err != nil {
			return res, err
		}
		res = append(res, hook)
	}
	return res, nil
}

func (d *DB) Remove(name string, convID chat1.ConvIDStr) error {
	log.Infof("Deleting webhooks [%s: %+v]", name, convID)
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(sqlDeleteHook, convID, name)
		return err
	})
}
