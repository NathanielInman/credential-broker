package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
	"github.com/nate/credential-broker/prompt"
	"github.com/nate/credential-broker/store"
)

type Server struct {
	Broker *model.Broker
	DB     *store.Store
	Mux    *http.ServeMux
}

func Start() error {
	var broker *model.Broker

	if _, err := os.Stat("broker.json"); os.IsNotExist(err) {
		broker = model.DefaultBrokerPtr()
		if err := askStrategies(broker); err != nil {
			return err
		}
		data, _ := json.MarshalIndent(broker, "", "  ")
		if err := os.WriteFile("broker.json", data, 0644); err != nil {
			return err
		}
	} else {
		data, err := os.ReadFile("broker.json")
		if err != nil {
			return err
		}
		broker = &model.Broker{}
		if err := json.Unmarshal(data, broker); err != nil {
			return err
		}
	}

	db, err := store.New("./data/broker.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	db.StartCleanup(60 * time.Second)

	srv := &Server{
		Broker: broker,
		DB:     db,
		Mux:    http.NewServeMux(),
	}

	srv.registerRoutes()

	// Start pending wipe checker
	go srv.wipeChecker()

	printBanner(broker)

	addr := fmt.Sprintf(":%d", broker.Port)
	fmt.Printf("Listening on %s\n", addr)
	return http.ListenAndServe(addr, srv.Mux)
}

func (s *Server) registerRoutes() {
	// Auth routes (no auth middleware)
	s.Mux.HandleFunc("POST /authSecure", s.middleware(s.handleAuthSecure))
	s.Mux.HandleFunc("POST /authIdentify", s.middleware(s.handleAuthIdentify))
	s.Mux.HandleFunc("POST /authChallenge", s.middleware(s.handleAuthChallenge))
	s.Mux.HandleFunc("POST /auth2FAInitialize", s.middleware(s.handleAuth2FAInitialize))
	s.Mux.HandleFunc("POST /auth2FAValidate", s.middleware(s.handleAuth2FAValidate))

	// User initialize (special auth - uses session but not full auth)
	s.Mux.HandleFunc("POST /userInitialize", s.middleware(s.handleUserInitialize))

	// Authenticated routes
	s.Mux.HandleFunc("POST /userAdd", s.middleware(s.authenticated(s.handleUserAdd)))
	s.Mux.HandleFunc("POST /userGet", s.middleware(s.authenticated(s.handleUserGet)))
	s.Mux.HandleFunc("POST /userGetAll", s.middleware(s.authenticated(s.handleUserGetAll)))
	s.Mux.HandleFunc("POST /userModify", s.middleware(s.authenticated(s.handleUserModify)))
	s.Mux.HandleFunc("POST /userDelete", s.middleware(s.authenticated(s.handleUserDelete)))
	s.Mux.HandleFunc("POST /scopeAdd", s.middleware(s.authenticated(s.handleScopeAdd)))
	s.Mux.HandleFunc("POST /scopeGet", s.middleware(s.authenticated(s.handleScopeGet)))
	s.Mux.HandleFunc("POST /scopeGetAll", s.middleware(s.authenticated(s.handleScopeGetAll)))
	s.Mux.HandleFunc("POST /scopeModify", s.middleware(s.authenticated(s.handleScopeModify)))
	s.Mux.HandleFunc("POST /scopeDelete", s.middleware(s.authenticated(s.handleScopeDelete)))
	s.Mux.HandleFunc("POST /secretAdd", s.middleware(s.authenticated(s.handleSecretAdd)))
	s.Mux.HandleFunc("POST /secretModify", s.middleware(s.authenticated(s.handleSecretModify)))
	s.Mux.HandleFunc("POST /secretDelete", s.middleware(s.authenticated(s.handleSecretDelete)))
	s.Mux.HandleFunc("POST /wipe", s.middleware(s.authenticated(s.handleWipe)))
	s.Mux.HandleFunc("POST /wipeCancel", s.middleware(s.authenticated(s.handleWipeCancel)))
}

// middleware adds common request processing (IP extraction, logging helper)
func (s *Server) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read body
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()

		// Store body in context for downstream handlers
		ctx := internal.WithBody(r.Context(), string(body))

		// Store server ref in context
		ctx = internal.WithServer(ctx, s.DB, s.Broker)

		r = r.WithContext(ctx)
		next(w, r)
	}
}

func (s *Server) wipeChecker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		data, err := s.DB.Get("config", "pendingWipe")
		if err != nil || data == nil {
			continue
		}

		var wipe struct {
			TriggeredAt int64  `json:"triggeredAt"`
			TriggeredBy string `json:"triggeredBy"`
			ExpiresAt   int64  `json:"expiresAt"`
		}
		if err := json.Unmarshal(data, &wipe); err != nil {
			continue
		}

		if time.Now().UnixMilli() >= wipe.ExpiresAt {
			// Execute wipe
			usersData, _ := s.DB.Get("config", "users")
			var users []struct {
				Name string `json:"name"`
			}
			if usersData != nil {
				json.Unmarshal(usersData, &users)
			}

			for _, u := range users {
				s.DB.Delete("users", u.Name)
			}
			s.DB.Set("config", "users", []byte("[]"), 0)
			s.DB.Delete("config", "pendingWipe")
			color.Yellow("Pending wipe executed - all users removed")
		}
	}
}

func askStrategies(broker *model.Broker) error {
	fmt.Println("Configure broker strategies:")
	for i := range broker.Strategies {
		val, err := prompt.Confirm(fmt.Sprintf("  %s?", broker.Strategies[i].Name))
		if err != nil {
			return err
		}
		broker.Strategies[i].Value = val
	}

	fmt.Println("\nEmail configuration:")
	service, err := prompt.Select("Email service", []string{"None", "SMTP"})
	if err != nil {
		return err
	}
	broker.EmailService = service

	if service == "SMTP" {
		host, _ := prompt.Text("SMTP Host")
		broker.EmailTransport.Host = host
		portStr, _ := prompt.Text("SMTP Port")
		fmt.Sscanf(portStr, "%d", &broker.EmailTransport.Port)
		broker.EmailTransport.TLS = true
		user, _ := prompt.Text("SMTP Username")
		broker.EmailTransport.Auth.User = user
		broker.EmailTransport.Auth.Type = "login"
		pass, _ := prompt.Password("SMTP Password")
		broker.EmailTransport.Auth.Pass = pass
	}

	return nil
}

func printBanner(broker *model.Broker) {
	blue := color.New(color.FgBlue).SprintFunc()
	fmt.Println(blue("         ./*.           "))
	fmt.Println(blue("     /@@@@@@@@@@.       "))
	fmt.Println(blue("   ,@@@@,    /@@@@      "))
	fmt.Println(blue("  .@@@.        *@@@     "))
	fmt.Println(blue("  @@@,          &@@%    "))
	fmt.Println(blue("  @@@.          %@@%    "))
	fmt.Println(blue("  @@@.          %@@%    "))
	fmt.Printf("%s v0.1.0\n", blue("@@@@@@@@@@@@@@@@@@@@@@, "))
	fmt.Printf("%s :%d\n", blue("@@@@@@@@@@%%@@@@@@@@@@, "), broker.Port)

	for i, s := range broker.Strategies {
		if i < 5 {
			status := color.RedString("OFF")
			if s.Value {
				status = color.GreenString("ON")
			}
			fmt.Printf("%s %s: %s\n", blue("@@@@@@@@@@@@@@@@@@@@@@, "), s.Name, status)
		}
	}

	fmt.Printf("Session TTL: %s\n", time.Duration(broker.SessionTTL)*time.Millisecond)
	fmt.Printf("2FA TTL: %s\n", time.Duration(broker.TwoFactorTTL)*time.Millisecond)
	fmt.Printf("Email: %s\n", broker.EmailService)
}

// Helper to get IP from request
func getIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
