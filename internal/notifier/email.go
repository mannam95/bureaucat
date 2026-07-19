package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
)

// SMTPConfig holds SMTP settings, typically loaded from environment variables.
// The email notifier is enabled only when Host, Port and From are all set.
type SMTPConfig struct {
	Host     string // SMTP_HOST
	Port     string // SMTP_PORT, e.g. "587"
	Username string // SMTP_USERNAME (optional; omit for unauthenticated relays)
	Password string // SMTP_PASSWORD
	From     string // SMTP_FROM_ADDRESS, e.g. "Bureaucat <no-reply@example.com>"
}

// Enabled reports whether the minimum configuration to send mail is present.
func (c SMTPConfig) Enabled() bool {
	return c.Host != "" && c.Port != "" && c.From != ""
}

// EmailNotifier delivers notifications as plain-text emails over SMTP.
type EmailNotifier struct {
	cfg SMTPConfig
}

// NewEmailNotifier creates an SMTP-backed notifier.
func NewEmailNotifier(cfg SMTPConfig) *EmailNotifier {
	return &EmailNotifier{cfg: cfg}
}

func (e *EmailNotifier) Name() string { return "email" }

// Send composes and delivers the notification email to recipientEmail.
func (e *EmailNotifier) Send(_ context.Context, recipientEmail string, n Notification) error {
	subject, body := renderEmail(n)
	msg := buildMessage(e.cfg.From, recipientEmail, subject, body)
	addr := net.JoinHostPort(e.cfg.Host, e.cfg.Port)
	from := senderAddress(e.cfg.From)

	var auth smtp.Auth
	if e.cfg.Username != "" {
		auth = newSMTPAuth(e.cfg.Username, e.cfg.Password)
	}

	// Port 465 expects TLS from the first byte. Other ports start in plaintext
	// and smtp.SendMail upgrades via STARTTLS when the server advertises it.
	if e.cfg.Port == "465" {
		return e.sendImplicitTLS(addr, auth, from, recipientEmail, msg)
	}
	if err := smtp.SendMail(addr, auth, from, []string{recipientEmail}, msg); err != nil {
		return fmt.Errorf("smtp send to %s: %w", recipientEmail, err)
	}
	return nil
}

// sendImplicitTLS handles the port-465 case, where the connection is TLS-wrapped
// from the start rather than upgraded with STARTTLS.
func (e *EmailNotifier) sendImplicitTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.cfg.Host})
	if err != nil {
		return fmt.Errorf("smtp tls dial %s: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, e.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

// renderEmail builds the subject and plain-text body for a notification.
func renderEmail(n Notification) (subject, body string) {
	ref := fmt.Sprintf("%s-%d", n.ProjectKey, n.TaskNumber)
	switch n.Event {
	case EventTaskAssigned:
		subject = fmt.Sprintf("You were assigned to %s: %s", ref, n.TaskTitle)
		body = fmt.Sprintf("%s assigned you to %s %q.", n.ActorName, ref, n.TaskTitle)
	case EventMentioned:
		subject = fmt.Sprintf("%s mentioned you on %s", n.ActorName, ref)
		body = fmt.Sprintf("%s mentioned you on %s %q.", n.ActorName, ref, n.TaskTitle)
	case EventCommented:
		subject = fmt.Sprintf("%s commented on %s", n.ActorName, ref)
		body = fmt.Sprintf("%s commented on %s %q.", n.ActorName, ref, n.TaskTitle)
	default:
		subject = fmt.Sprintf("Update on %s", ref)
		body = fmt.Sprintf("There was an update on %s %q.", ref, n.TaskTitle)
	}
	body += fmt.Sprintf("\n\nView it here:\n%s\n", n.TaskURL())
	return subject, body
}

// buildMessage assembles a minimal RFC 5322 plain-text message.
func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}

// smtpAuth implements smtp.Auth, choosing PLAIN or LOGIN based on what the server
// advertises. Go's net/smtp only ships PLAIN and CRAM-MD5, but some providers
// (Microsoft Exchange / Azure Communication Services) only accept AUTH LOGIN.
type smtpAuth struct {
	username string
	password string

	mech      string
	loginStep int
}

func newSMTPAuth(username, password string) *smtpAuth {
	return &smtpAuth{username: username, password: password}
}

func (a *smtpAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// Never send credentials over an unencrypted link (except to a loopback
	// relay). By the time Auth runs on port 587 the STARTTLS upgrade is done,
	// so server.TLS is true for real providers.
	if !server.TLS && !isLoopback(server.Name) {
		return "", nil, fmt.Errorf("smtp: refusing to authenticate to %q over an unencrypted connection", server.Name)
	}
	if advertises(server.Auth, "PLAIN") {
		a.mech = "PLAIN"
		return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
	}
	if advertises(server.Auth, "LOGIN") {
		a.mech = "LOGIN"
		return "LOGIN", nil, nil
	}
	// Nothing recognised was advertised; PLAIN is the safest default to attempt.
	a.mech = "PLAIN"
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a *smtpAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more || a.mech != "LOGIN" {
		return nil, nil
	}
	// LOGIN sends the username at the first challenge and the password at the
	// second, regardless of the prompt wording.
	a.loginStep++
	switch a.loginStep {
	case 1:
		return []byte(a.username), nil
	case 2:
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("smtp login: unexpected server challenge %q", fromServer)
	}
}

func advertises(mechs []string, name string) bool {
	for _, m := range mechs {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// senderAddress extracts the bare address from a "Name <addr>" From value, which
// is what the SMTP envelope (MAIL FROM) requires.
func senderAddress(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		if j := strings.Index(from[i:], ">"); j > 0 {
			return strings.TrimSpace(from[i+1 : i+j])
		}
	}
	return strings.TrimSpace(from)
}
