package mail

import (
	"fmt"

	"github.com/nate/credential-broker/model"
	"github.com/wneessen/go-mail"
)

// Send sends an email notification. If broker.EmailService is "None", returns silently.
// Parameters:
//   - broker: the broker config with email transport settings
//   - to: recipient email address
//   - title: subject suffix (prefixed with "Credential Broker - ")
//   - body: email body (sent as both text and HTML)
func Send(broker *model.Broker, to, title, body string) error {
	if broker == nil || broker.EmailService == "None" {
		return nil
	}

	m := mail.NewMsg()

	if err := m.From(broker.EmailTransport.Auth.User); err != nil {
		return fmt.Errorf("failed to set from address: %w", err)
	}
	if err := m.To(to); err != nil {
		return fmt.Errorf("failed to set to address: %w", err)
	}

	m.Subject("Credential Broker - " + title)
	m.SetBodyString(mail.TypeTextPlain, body)
	m.AddAlternativeString(mail.TypeTextHTML, body)

	opts := []mail.Option{
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithUsername(broker.EmailTransport.Auth.User),
		mail.WithPassword(broker.EmailTransport.Auth.Pass),
		mail.WithPort(broker.EmailTransport.Port),
	}

	if broker.EmailTransport.TLS {
		opts = append(opts, mail.WithTLSPortPolicy(mail.TLSMandatory))
	}

	c, err := mail.NewClient(broker.EmailTransport.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create mail client: %w", err)
	}

	if err := c.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
