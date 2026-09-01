package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// deliverEmail sends the event over SMTP.
//
// Config keys: smtp_host, smtp_port (default 587), smtp_user, smtp_password, from, to
// (comma-separated). Authentication is optional, because a box relaying through a
// local unauthenticated MTA on port 25 is a completely normal self-hosted setup.
//
// Transport security is chosen by port: 465 is implicit TLS, everything else attempts
// STARTTLS and refuses to send credentials over a connection that did not upgrade.
// Sending a password in the clear because a server declined STARTTLS is exactly the
// failure this should be loud about rather than quietly tolerate.
func (n *Notifier) deliverEmail(ctx context.Context, t Target, ev RunFailedEvent) error {
	host := t.Config["smtp_host"]
	if host == "" {
		return errors.New("email target has no smtp_host")
	}
	to := splitList(t.Config["to"])
	if len(to) == 0 {
		return errors.New("email target has no recipients")
	}
	from := t.Config["from"]
	if from == "" {
		from = "croncompose@" + host
	}
	port := t.Config["smtp_port"]
	if port == "" {
		port = "587"
	}
	addr := net.JoinHostPort(host, port)

	msg := buildMessage(from, to, ev)

	client, err := dialSMTP(ctx, addr, host, port == "465")
	if err != nil {
		return err
	}
	defer client.Close()

	if port != "465" {
		if okTLS, _ := client.Extension("STARTTLS"); okTLS {
			if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		} else if t.Config["smtp_password"] != "" {
			return errors.New("server does not offer STARTTLS; refusing to send credentials in the clear")
		}
	}

	if user := t.Config["smtp_user"]; user != "" {
		if err := client.Auth(smtp.PlainAuth("", user, t.Config["smtp_password"], host)); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// dialSMTP connects with a deadline drawn from the context, so a black-holed SMTP port
// cannot hold a notification goroutine open indefinitely.
func dialSMTP(ctx context.Context, addr, host string, implicitTLS bool) (*smtp.Client, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	if implicitTLS {
		conn, err := tls.DialWithDialer(&d, "tcp", addr,
			&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, host)
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, host)
}

// buildMessage assembles a plain-text RFC 5322 message. Plain text on purpose: these
// go to phones and to terminal mail readers, and the content is a short status plus a
// block of program output.
func buildMessage(from string, to []string, ev RunFailedEvent) []byte {
	subject := fmt.Sprintf("[CronCompose] %s %s on %s",
		ev.Status, nameOr(ev.JobName, ev.JobID), nameOr(ev.ServerName, ev.ServerID))

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "Job:      %s\n", nameOr(ev.JobName, ev.JobID))
	fmt.Fprintf(&b, "Server:   %s\n", nameOr(ev.ServerName, ev.ServerID))
	fmt.Fprintf(&b, "Status:   %s\n", ev.Status)
	fmt.Fprintf(&b, "Exit:     %d\n", ev.ExitCode)
	fmt.Fprintf(&b, "Duration: %s\n", humanMillis(ev.DurationMs))
	if ev.RunURL != "" {
		fmt.Fprintf(&b, "Run:      %s\n", ev.RunURL)
	}
	if ev.Error != "" {
		b.WriteString("\n")
		b.WriteString(truncate(ev.Error, 4000))
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n'
	}) {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
