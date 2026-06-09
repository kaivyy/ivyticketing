package templates

import (
	"bytes"
	"fmt"
	"html/template"
)

// Notification type constants — defined here to avoid import cycles.
// The notifications package re-exports these same string values.
const (
	NotifOrderCreated      = "order.created"
	NotifPaymentPaid       = "payment.paid"
	NotifPaymentExpired    = "payment.expired"
	NotifQueueAllowed      = "queue.allowed"
	NotifBallotWinner      = "ballot.winner"
	NotifBallotNotSelected = "ballot.not_selected"
	NotifWaitlistPromoted  = "waitlist.promoted"
)

// TemplateData holds all fields any notification template might need.
// Populate only the fields available at the trigger point.
type TemplateData struct {
	ParticipantName  string
	ParticipantEmail string
	EventName        string
	CategoryName     string
	OrderID          string
	OrderNumber      string
	TotalAmount      string
	CheckoutURL      string
	PaymentDeadline  string
	QueuePosition    int
	BallotDrawID     string
	WaitlistPosition int
}

// RenderResult holds the rendered notification content.
type RenderResult struct {
	Subject  string
	HTMLBody string
	TextBody string
}

type tmplDef struct {
	subject  string
	htmlTmpl string
}

var defs = map[string]tmplDef{
	NotifOrderCreated: {
		subject: "Pesanan Anda Berhasil Dibuat — {{.OrderNumber}}",
		htmlTmpl: `<h2>Pesanan Berhasil Dibuat</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Pesanan Anda untuk <strong>{{.EventName}}</strong> dengan nomor <strong>{{.OrderNumber}}</strong> telah berhasil dibuat.</p>
<p>Total pembayaran: <strong>{{.TotalAmount}}</strong></p>
{{if .CheckoutURL}}<p>Selesaikan pembayaran sebelum <strong>{{.PaymentDeadline}}</strong>: <a href="{{.CheckoutURL}}">Bayar Sekarang</a></p>{{end}}
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifPaymentPaid: {
		subject: "Pembayaran Dikonfirmasi — {{.OrderNumber}}",
		htmlTmpl: `<h2>Pembayaran Berhasil</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Pembayaran untuk pesanan <strong>{{.OrderNumber}}</strong> — <strong>{{.EventName}}</strong> telah dikonfirmasi.</p>
<p>Total: <strong>{{.TotalAmount}}</strong></p>
<p>Tiket Anda telah diterbitkan. Sampai jumpa di acara!</p>
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifPaymentExpired: {
		subject: "Pesanan Kadaluarsa — {{.OrderNumber}}",
		htmlTmpl: `<h2>Pesanan Kadaluarsa</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Sayang sekali, pesanan Anda <strong>{{.OrderNumber}}</strong> untuk <strong>{{.EventName}}</strong> telah kadaluarsa karena pembayaran tidak diselesaikan tepat waktu.</p>
<p>Silakan coba lagi jika tiket masih tersedia.</p>
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifQueueAllowed: {
		subject: "Giliran Anda! Selesaikan Pembelian Sekarang — {{.EventName}}",
		htmlTmpl: `<h2>Anda Diizinkan Masuk</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Giliran Anda untuk membeli tiket <strong>{{.EventName}}</strong> telah tiba!</p>
{{if .CheckoutURL}}<p>Segera selesaikan pembelian sebelum waktu habis: <a href="{{.CheckoutURL}}">Beli Sekarang</a></p>{{end}}
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifBallotWinner: {
		subject: "Selamat! Anda Terpilih dalam Ballot — {{.EventName}}",
		htmlTmpl: `<h2>Anda Terpilih!</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Selamat! Anda terpilih sebagai pemenang ballot untuk <strong>{{.EventName}}</strong>.</p>
{{if .CheckoutURL}}<p>Segera selesaikan pembelian tiket Anda: <a href="{{.CheckoutURL}}">Beli Sekarang</a></p>{{end}}
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifBallotNotSelected: {
		subject: "Hasil Ballot — {{.EventName}}",
		htmlTmpl: `<h2>Hasil Ballot</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Terima kasih telah mendaftar ballot untuk <strong>{{.EventName}}</strong>. Sayangnya, Anda tidak terpilih dalam pengundian kali ini.</p>
<p>Pantau terus informasi terbaru untuk kesempatan lainnya.</p>
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
	NotifWaitlistPromoted: {
		subject: "Anda Dipromosikan dari Waitlist — {{.EventName}}",
		htmlTmpl: `<h2>Slot Tersedia untuk Anda!</h2>
<p>Halo {{.ParticipantName}},</p>
<p>Kabar baik! Slot untuk <strong>{{.EventName}}</strong> kini tersedia untuk Anda dari waitlist.</p>
{{if .CheckoutURL}}<p>Segera selesaikan pembelian sebelum slot diambil orang lain: <a href="{{.CheckoutURL}}">Beli Sekarang</a></p>{{end}}
<p>Terima kasih,<br>Tim IvyTicketing</p>`,
	},
}

// Render renders the subject and HTML body for the given notification type and data.
// Returns an error if the type is unknown or template execution fails.
func Render(typ string, data TemplateData) (RenderResult, error) {
	def, ok := defs[typ]
	if !ok {
		return RenderResult{}, fmt.Errorf("unknown notification type: %s", typ)
	}

	subj, err := renderString(def.subject, data)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render subject for %s: %w", typ, err)
	}
	body, err := renderString(def.htmlTmpl, data)
	if err != nil {
		return RenderResult{}, fmt.Errorf("render html for %s: %w", typ, err)
	}

	return RenderResult{Subject: subj, HTMLBody: body, TextBody: stripHTML(body)}, nil
}

func renderString(tmplStr string, data TemplateData) (string, error) {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// stripHTML is a minimal plain-text fallback — strips HTML tags.
func stripHTML(s string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
