package mf

import (
	"fmt"
	"os"
	"strings"

	"github.com/zovgo/format"
	"github.com/zovgo/maxforward/internal"
	"github.com/zovgo/maxproto"
	"github.com/zovgo/maxproto/packet"
	"github.com/zovgo/maxproto/protocol"
)

func (f *Forwarder) dialMaxUnsafe(att int64) (*maxproto.Client, error) {
	//state must be locked here
	f.conf.Logger.Info("dialing max...", "attempt", att)
	conf := maxproto.DialConfig{
		Token:     os.Getenv(f.conf.Max.TokenEnvironment),
		DeviceID:  f.device,
		ChatCount: f.conf.ChatCount,
	}
	cl, err := conf.DialContext(f.ctx, false)
	if err != nil {
		return nil, err
	}
	f.conf.Logger.Info("dialed max.", "attempt", att)
	f.state.V.cl = cl
	return cl, nil
}

func (f *Forwarder) onMessage(cl *maxproto.Client) func(pk *packet.ReceiveMessage) {
	return func(pk *packet.ReceiveMessage) {
		f.conf.Logger.Debug("received max message", "chat", pk.ChatID, "msg", fmt.Sprintln(pk.Message.Text))

		msg, ok := f.buildMessage(cl, pk)
		if !ok {
			return
		}
		err := f.sendTelegramMessage(f.conf.Telegram.GroupID, msg)
		if err != nil {
			f.conf.Logger.Error("send telegram message", "err", err.Error())
		}
	}
}

var messageFormat = internal.JoinNewLines(
	"[CONTACT]:",
	"[CONTENT]",
)

func (f *Forwarder) buildMessage(cl *maxproto.Client, pk *packet.ReceiveMessage) (string, bool) {
	if pk.Message.Type != "USER" {
		f.conf.Logger.Warn("not a user message", "type", pk.Message.Type)
		return "", false
	}
	if pk.ChatID != f.conf.Max.GroupID {
		f.conf.Logger.Warn("other chat id", "id", pk.ChatID)
		return "", false
	}
	c, ok := cl.GetContact(pk.Message.Sender)
	if !ok {
		f.conf.Logger.Error("contact not found", "sender", pk.Message.Sender)
		return "", false
	}
	return format.F(messageFormat).
		With("CONTACT", contactName(c)).
		WithFinal("CONTENT", messageContent(pk)), true
}

func messageContent(pk *packet.ReceiveMessage) string {
	if str := attachesString(pk); str != "" {
		return str
	}
	if pk.Message.Text != "" {
		return pk.Message.Text
	}
	return "(empty message)"
}

func attachesString(pk *packet.ReceiveMessage) string {
	str := ""
	for i, a := range pk.Message.Attaches {
		if i != 0 && i != len(pk.Message.Attaches)-1 {
			str += "\n"
		}
		if a.Type == "PHOTO" {
			str += fmt.Sprintf(`(<a href="%s">%s</a>)`, a.BaseUrl, "photo")
			continue
		}
		str += "(" + strings.ToLower(a.Type) + ")"
	}
	return str
}

func contactName(c protocol.Contact) string {
	fall := "undefined"
	for _, n := range c.Names {
		if n.Type == "ONEME" {
			return cleanContactName(n)
		}
		fall = cleanContactName(n)
	}
	return fall
}

func cleanContactName(n protocol.ContactName) string {
	if n.LastName != "" {
		return n.FirstName + " " + n.LastName
	}
	return n.FirstName
}
