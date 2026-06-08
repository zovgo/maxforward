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

		msg := f.buildMessage(cl, pk)
		if len(msg) == 0 {
			return
		}
		for i, m := range msg {
			err := f.sendTelegramMessage(f.conf.Telegram.GroupID, m)
			if err != nil {
				f.conf.Logger.Error("send telegram message", "err", err.Error(), "i", i)
			}
		}
	}
}

var messageFormat = internal.JoinNewLines(
	"[CHAT] [CONTACT]:",
	"[CONTENT]",
)

func (f *Forwarder) buildMessage(cl *maxproto.Client, pk *packet.ReceiveMessage) []string {
	if pk.Message.Type != "USER" {
		f.conf.Logger.Warn("not a user message", "type", pk.Message.Type)
		return nil
	}
	if f.conf.Max.GroupID != 0 && pk.ChatID != f.conf.Max.GroupID {
		f.conf.Logger.Warn("other chat id", "id", pk.ChatID)
		return nil
	}
	if len(pk.Message.Attaches) > 5 {
		return []string{"too many attachments..."}
	}
	c, ok := cl.Contact(pk.Message.Sender)
	if !ok {
		f.conf.Logger.Error("contact not found", "sender", pk.Message.Sender)
		return nil
	}
	return formatMessages(cl, c, pk)
}

func formatMessages(cl *maxproto.Client, c protocol.Contact, pk *packet.ReceiveMessage) []string {
	x := make([]string, 0, len(pk.Message.Attaches))
	contact := contactName(c)

	for _, content := range messageContent(pk) {
		x = append(x, format.F(messageFormat).
			With("CONTACT", contact).
			With("CONTENT", content).
			WithFinal("CHAT", chat(cl, contact, pk)))
	}
	return x
}

func chat(cl *maxproto.Client, contact string, pk *packet.ReceiveMessage) string {
	str := "no chat"
	if ch, ok := cl.Chat(pk.ChatID); ok {
		switch ch.Type {
		case "CHAT":
			str = "(" + ch.Title + ")"
		case "DIALOG": //dm
			str = "dm"
			if name := chatDialogName(cl, ch); name != "" && name != contact {
				str += " " + "(" + name + ")"
			}
		default:
			str = "(" + strings.ToLower(ch.Type) + ")"
		}
	}
	return str
}

func chatDialogName(cl *maxproto.Client, ch protocol.Chat) string {
	str := ""
	for id := range ch.Participants {
		if id == cl.Profile().Contact.ID {
			continue
		}
		if c, ok := cl.Contact(id); ok {
			str = contactName(c)
			break
		}
	}
	return str
}

func messageContent(pk *packet.ReceiveMessage) []string {
	a := attachesMessages(pk)
	if len(a) != 0 {
		return a
	}
	if pk.Message.Text != "" {
		return []string{pk.Message.Text}
	}
	return []string{"(empty message)"}
}

type attach struct {
	link bool
	text string
}

func attachesMessages(pk *packet.ReceiveMessage) []string {
	a := buildAttaches(pk.Message)
	if len(a) == 0 {
		return nil
	}
	links := make([]string, 0, len(a))
	texts := make([]string, 0, len(a))

	for _, m := range a {
		if m.link {
			links = append(links, m.text)
			continue
		}
		texts = append(texts, m.text)
	}
	result := append(make([]string, 0, len(links)+1), links...)
	if len(texts) > 0 {
		result = append(result, strings.Join(texts, "\n"))
	}
	return result
}

func buildAttaches(m *protocol.Message) []attach {
	x := make([]attach, 0, len(m.Attaches))
	for _, a := range m.Attaches {
		att := attach{}
		if a.BaseURL == "" && a.URL == "" {
			att.text += "(" + strings.ToLower(a.Type) + ")"
			x = append(x, att)
			continue
		}
		att.link = true
		if a.BaseURL == "" {
			a.BaseURL = a.URL
		}
		att.text += fmt.Sprintf(`<a href="%s">[%s]</a>`, a.BaseURL, attachType(a))
		x = append(x, att)
	}
	return x
}

func attachType(a protocol.Attach) string {
	switch a.Type {
	case "SHARE":
		return "link"
	}
	return strings.ToLower(a.Type)
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
