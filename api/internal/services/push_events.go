package services

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Web Push events and the notification payload (docs/specs/2026-11-17-web-push.md, contract v1). The payload follows
// the Declarative Web Push shape and the golden file of go-common/webpush/testdata/notification-v1.json; a change here
// is a contract change (bump PushPayloadVersion). Nothing fiscal travels in it: no value, issuer, recipient, CNPJ,
// number or SEFAZ reason. The access key appears only in `navigate`, inside the end-to-end encrypted body.
const (
	PushEventAuthorized   = WebhookEventAuthorized
	PushEventRejected     = WebhookEventRejected
	PushEventFailed       = WebhookEventFailed
	PushEventDistribution = "distribution.received"

	// PushPayloadVersion is data.v of the notification; the service worker ignores a version it does not know.
	PushPayloadVersion = 1
	// PushPayloadBudget is the ceiling of one payload, far under webpush.MaxPlaintext.
	PushPayloadBudget = 1024
	// PushCompanyNameMax bounds the company name in a title (the lock screen shows it).
	PushCompanyNameMax = 30
	// PushResultWindow and PushDistributionWindow group a company's events of one type into one push.
	PushResultWindow       = time.Minute
	PushDistributionWindow = 15 * time.Minute
	// MaxPushRecipientsPerPage is one read of a company's preference rows; MaxPushRecipientPages bounds the
	// fan-out of one event (ceiling: 1,000 people with a preference in one company).
	MaxPushRecipientsPerPage = 100
	MaxPushRecipientPages    = 10

	pushWebPushMagic = 8030
	pushLang         = "pt-BR"
)

// PushEventTypes is every type a person may tick, in the order the screen lists them.
var PushEventTypes = []string{PushEventDistribution, PushEventAuthorized, PushEventRejected, PushEventFailed}

// PushDefaultEvents is what a new device starts with: only the documents that arrived through distribution. The
// results of one's own emissions start off (an NFC-e till would otherwise notify on every sale).
var PushDefaultEvents = []string{PushEventDistribution}

// PushSendJob is the event of the push-sender Lambda: the notification already rendered, and who gets it. The Lambda
// reads the devices of each recipient and sends; it decides nothing about content.
type PushSendJob struct {
	EventID    string   `json:"event_id"`
	Type       string   `json:"type"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Navigate   string   `json:"navigate"`
	Tag        string   `json:"tag"`
	Topic      string   `json:"topic"`
	TTLSeconds int      `json:"ttl_seconds"`
	Urgency    string   `json:"urgency"`
	Recipients []string `json:"recipients"`
}

type pushData struct {
	V    int    `json:"v"`
	ID   string `json:"id"`
	Type string `json:"type"`
}

type pushNotification struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Navigate string   `json:"navigate"`
	Tag      string   `json:"tag"`
	Lang     string   `json:"lang"`
	Silent   bool     `json:"silent"`
	Data     pushData `json:"data"`
}

type pushEnvelope struct {
	WebPush      int              `json:"web_push"`
	Notification pushNotification `json:"notification"`
}

// BuildPushPayload renders the notification of a job and refuses one over PushPayloadBudget or whose link is not a
// path of the app itself.
func BuildPushPayload(j PushSendJob) ([]byte, error) {
	if !strings.HasPrefix(j.Navigate, "/") || strings.HasPrefix(j.Navigate, "//") || strings.Contains(j.Navigate, "\\") {
		return nil, fmt.Errorf("push: navigate must be a path of the app")
	}
	body, err := json.Marshal(pushEnvelope{WebPush: pushWebPushMagic, Notification: pushNotification{
		Title: j.Title, Body: j.Body, Navigate: j.Navigate, Tag: j.Tag, Lang: pushLang,
		Data: pushData{V: PushPayloadVersion, ID: j.EventID, Type: j.Type},
	}})
	if err != nil {
		return nil, err
	}
	if len(body) > PushPayloadBudget {
		return nil, fmt.Errorf("push: payload is %d bytes, over the %d budget", len(body), PushPayloadBudget)
	}
	return body, nil
}

// pushDocLabels names the document in a title and its grammatical gender ("NF-e autorizada", "CT-e autorizado").
var pushDocLabels = map[string]struct {
	Label     string
	Masculine bool
}{
	MeterNFe: {"NF-e", false}, MeterNFCe: {"NFC-e", false}, MeterCTe: {"CT-e", true}, MeterMDFe: {"MDF-e", true},
	MeterNFSe: {"NFS-e", false}, MeterNFCom: {"NFCom", false}, MeterBPe: {"BP-e", true},
}

// PushDocumentLabel is the display name of a document type ("nfe" gives "NF-e"); empty for an unknown one.
func PushDocumentLabel(docType string) string { return pushDocLabels[docType].Label }

// pushDetailPaths is the UI detail route of each document and the query parameter that names it.
var pushDetailPaths = map[string]struct{ Path, Param string }{
	MeterNFe: {"/nfe/detail", "key"}, MeterNFCe: {"/nfce/detail", "key"}, MeterCTe: {"/cte/detail", "key"},
	MeterMDFe: {"/mdfe/detail", "key"}, MeterNFSe: {"/nfse/detail", "id"}, MeterNFCom: {"/nfcom/detail", "key"},
	MeterBPe: {"/bpe/detail", "key"},
}

// pushDistributionPaths is where the received documents of each type are listed.
var pushDistributionPaths = map[string]string{
	MeterNFe: "/nfe?tab=distribuicao", MeterCTe: "/cte/distributions", MeterMDFe: "/mdfe/distributions",
}

// pushNavigate builds the link the click opens. org names the company so the app can switch to it first.
func pushNavigate(eventType, docType, key, companyPK string) string {
	org := "org=" + url.QueryEscape(companyPK)
	if eventType == PushEventDistribution {
		path, ok := pushDistributionPaths[docType]
		if !ok {
			path = pushDistributionPaths[MeterNFe]
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return path + sep + org
	}
	d, ok := pushDetailPaths[docType]
	if !ok {
		return "/?" + org
	}
	return d.Path + "?" + d.Param + "=" + url.QueryEscape(key) + "&" + org
}

// pushTitle is "<what happened> · <company>", the company truncated to PushCompanyNameMax characters.
func pushTitle(eventType, docType, company string) string {
	var head string
	switch eventType {
	case PushEventDistribution:
		head = "Novos documentos recebidos"
	default:
		d, ok := pushDocLabels[docType]
		if !ok {
			d.Label = "Documento"
		}
		switch eventType {
		case PushEventAuthorized:
			head = d.Label + pushGender(d.Masculine, " autorizada", " autorizado")
		case PushEventRejected:
			head = d.Label + pushGender(d.Masculine, " rejeitada", " rejeitado")
		default:
			head = "Falha ao emitir " + d.Label
		}
	}
	if name := pushCompanyName(company); name != "" {
		return head + " · " + name
	}
	return head
}

func pushGender(masculine bool, feminine, masculineForm string) string {
	if masculine {
		return masculineForm
	}
	return feminine
}

// pushCompanyName trims the name to PushCompanyNameMax characters, with an ellipsis when it was cut.
func pushCompanyName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if utf8.RuneCountInString(name) <= PushCompanyNameMax {
		return name
	}
	r := []rune(name)
	return strings.TrimSpace(string(r[:PushCompanyNameMax-1])) + "…"
}

// pushBody is the generic line under the title.
func pushBody(eventType string) string {
	switch eventType {
	case PushEventAuthorized:
		return "Toque para abrir o documento."
	case PushEventRejected:
		return "Toque para ver o motivo."
	case PushEventFailed:
		return "Toque para ver o que aconteceu."
	default:
		return "Há documentos emitidos contra a sua empresa."
	}
}

// pushDelivery is the TTL and urgency of an event: a rejection is worth waking the device, a received document is not.
func pushDelivery(eventType string) (ttl time.Duration, urgency string) {
	switch eventType {
	case PushEventAuthorized:
		return 12 * time.Hour, "normal"
	case PushEventRejected, PushEventFailed:
		return 24 * time.Hour, "high"
	default:
		return 24 * time.Hour, "low"
	}
}

// pushTag groups what the device shows: one entry per company and type. The same digest is the Topic, so the push
// service also keeps only the latest while the device is offline.
func pushTag(eventType, companyPK string) (tag, topic string) {
	sum := sha256.Sum256([]byte(companyPK + "|" + eventType))
	return eventType + ":" + hex.EncodeToString(sum[:4]), base64.RawURLEncoding.EncodeToString(sum[:12])
}

// pushWindow is how long a company's events of one type are grouped into one push.
func pushWindow(eventType string) time.Duration {
	if eventType == PushEventDistribution {
		return PushDistributionWindow
	}
	return PushResultWindow
}

// pushEventOf maps a terminal document status to its push event type.
func pushEventOf(status string) (string, bool) {
	t, ok := webhookEventTypeOf[status]
	return t, ok
}

// pushDeviceLabel is a coarse label of the browser and system ("Chrome no Android") for the devices list. Only a
// coarse label: the full User-Agent identifies a person's machine and is not kept.
func pushDeviceLabel(userAgent string) string {
	ua := strings.ToLower(userAgent)
	browser := "Navegador"
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "samsungbrowser"):
		browser = "Samsung Internet"
	case strings.Contains(ua, "firefox") || strings.Contains(ua, "fxios"):
		browser = "Firefox"
	case strings.Contains(ua, "chrome") || strings.Contains(ua, "crios"):
		browser = "Chrome"
	case strings.Contains(ua, "safari"):
		browser = "Safari"
	}
	system := ""
	switch {
	case strings.Contains(ua, "android"):
		system = "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		system = "iOS"
	case strings.Contains(ua, "windows"):
		system = "Windows"
	case strings.Contains(ua, "mac os"):
		system = "macOS"
	case strings.Contains(ua, "linux"):
		system = "Linux"
	}
	if system == "" {
		return browser
	}
	return browser + " no " + system
}
