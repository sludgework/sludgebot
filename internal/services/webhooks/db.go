package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
	"github.com/keybase/managed-bots/base"
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
	var hookType string = "msghook"
	id, err := d.makeID(name, convID)
	if err != nil {
		return "", err
	}
	err = d.RunTxn(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`
			INSERT INTO hooks
			(id, name, conv_id, hook_type)
			VALUES
			(?, ?, ?, ?)
		`, id, name, convID, hookType); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

func (d *DB) GetHook(id string) (res Webhook, err error) {
	row := d.DB.QueryRow(`
		SELECT conv_id, name, hook_type FROM hooks WHERE id = ?
	`, id)
	if err := row.Scan(&res.ConvID, &res.Name, &res.HookType); err != nil {
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
	rows, err := d.DB.Query(`
		SELECT id, name, hook_type FROM hooks WHERE conv_id = ?
	`, convID)
	if err != nil {
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
	return d.RunTxn(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			DELETE FROM hooks WHERE conv_id = ? AND name = ?
		`, convID, name)
		return err
	})
}
