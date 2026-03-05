package model

// Strategy represents a broker abandonment/access strategy.
type Strategy struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

// EmailAuth holds authentication credentials for the email transport.
type EmailAuth struct {
	Type         string `json:"type"`                    // "login" or "OAuth2"
	User         string `json:"user"`
	Pass         string `json:"pass"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// EmailTransport configures the SMTP connection for sending emails.
type EmailTransport struct {
	Host string    `json:"host"`
	Port int       `json:"port"`
	TLS  bool      `json:"tls"`
	Auth EmailAuth `json:"auth"`
}

// Broker represents the server-side broker configuration.
type Broker struct {
	ExternalIP     string         `json:"externalIP"`
	Port           int            `json:"port"`
	ScopeNumber    int            `json:"scopeNumber"`
	UserNumber     int            `json:"userNumber"`
	LastAccess     string         `json:"lastAccess"`
	LastUser       string         `json:"lastUser"`
	Strategies     []Strategy     `json:"strategies"`
	EmailService   string         `json:"emailService"`   // "None" or "SMTP"
	EmailTransport EmailTransport `json:"emailTransport"`
	SessionTTL     int            `json:"sessionTTL"`     // milliseconds
	TwoFactorTTL   int            `json:"twoFactorTTL"`   // milliseconds
}

// DefaultBrokerPtr returns a pointer to a Broker with sensible defaults.
func DefaultBrokerPtr() *Broker {
	b := DefaultBroker()
	return &b
}

// DefaultBroker returns a Broker initialized with sensible defaults.
func DefaultBroker() Broker {
	return Broker{
		Port:         4000,
		SessionTTL:   300000,    // 5 minutes
		TwoFactorTTL: 43200000,  // 12 hours
		EmailService: "None",
		Strategies: []Strategy{
			{Name: "Oldest user acquires edit access", Value: false},
			{Name: "Revoker acquires abandoned scope access", Value: false},
			{Name: "All users allowed to revoke", Value: false},
			{Name: "First account gets access", Value: false},
			{Name: "Any user can cancel a wipe", Value: false},
		},
	}
}
