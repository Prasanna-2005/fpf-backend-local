package utility

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"time"

	"form_pf_backend/config"
)

// mailSession keeps one authenticated Gmail connection. A new conversation is a
// series of round trips, about half a second each, which is the multi-second send.
const (
	smtpKeepalive = 30 * time.Second
	smtpDeadline  = 20 * time.Second
)

// errSMTPNotStarted means the message had not been handed to Gmail yet, so a dead
// connection can be replaced and the send tried once more.
var errSMTPNotStarted = errors.New("smtp message not started")

var mailSession smtpSession

type smtpSession struct {
	mu        sync.Mutex
	client    *smtp.Client
	conn      net.Conn
	host      string
	user      string
	keepAlive sync.Once
}

func (s *smtpSession) send(cfg config.SMTPconfig, from string, to []string, msg []byte) error {
	// Held for the whole send. transmit, dial, and close use the connection without locking again.
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.transmit(cfg, from, to, msg)
	if err == nil {
		s.armKeepAlive()
		return nil
	}
	// A failure after MAIL FROM may mean Gmail already took the message.
	// Retry only when the message never started, which is what a dropped connection looks like.
	if !errors.Is(err, errSMTPNotStarted) {
		s.close()
		return err
	}
	s.close()
	if err = s.transmit(cfg, from, to, msg); err != nil {
		s.close()
		return err
	}
	s.armKeepAlive()
	return nil
}

func (s *smtpSession) transmit(cfg config.SMTPconfig, from string, to []string, msg []byte) error {
	if s.client == nil || s.host != cfg.SMTPHost || s.user != cfg.SMTPUser {
		if err := s.dial(cfg); err != nil {
			return err
		}
	}
	s.armDeadline()
	defer s.clearDeadline()
	if err := s.client.Mail(from); err != nil {
		return fmt.Errorf("%w: %w", errSMTPNotStarted, err)
	}
	for _, rcpt := range to {
		if err := s.client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := s.client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (s *smtpSession) dial(cfg config.SMTPconfig) error {
	s.close()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(cfg.SMTPHost, cfg.SMTPPort), 10*time.Second)
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpDeadline))
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	if err = client.Hello("localhost"); err != nil {
		client.Close()
		return err
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: cfg.SMTPHost}
		if err = client.StartTLS(tlsCfg); err != nil {
			client.Close()
			return err
		}
	}
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	if err = client.Auth(auth); err != nil {
		client.Close()
		return err
	}
	_ = conn.SetDeadline(time.Time{})
	s.client = client
	s.conn = conn
	s.host = cfg.SMTPHost
	s.user = cfg.SMTPUser
	return nil
}

func (s *smtpSession) close() {
	if s.client != nil {
		s.client.Close()
	}
	s.client = nil
	s.conn = nil
}

func (s *smtpSession) armDeadline() {
	if s.conn != nil {
		_ = s.conn.SetDeadline(time.Now().Add(smtpDeadline))
	}
}

func (s *smtpSession) clearDeadline() {
	if s.conn != nil {
		_ = s.conn.SetDeadline(time.Time{})
	}
}

func (s *smtpSession) armKeepAlive() {
	s.keepAlive.Do(func() {
		go func() {
			ticker := time.NewTicker(smtpKeepalive)
			defer ticker.Stop()
			for range ticker.C {
				s.mu.Lock() // same lock as send, so a keep-alive cannot run during a message
				if s.client != nil {
					s.armDeadline()
					err := s.client.Noop()
					s.clearDeadline()
					if err != nil {
						s.close()
					}
				}
				s.mu.Unlock()
			}
		}()
	})
}
