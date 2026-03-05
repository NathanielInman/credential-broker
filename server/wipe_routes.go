package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/mail"
	"github.com/nate/credential-broker/model"
)

const wipeDelayMs = 24 * 60 * 60 * 1000 // 24 hours in ms

// POST /wipe - Initiate user wipe
func (s *Server) handleWipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	// Check for existing pending wipe
	wipeData, _ := s.DB.Get("config", "pendingWipe")
	if wipeData != nil {
		internal.Log(ip, name, "Wipe (Already Pending)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{
			"error": "A wipe is already pending. Cancel it first or wait for it to execute.",
		})
		return
	}

	canCancelWipe := s.Broker.Strategies[4].Value

	if canCancelWipe {
		// Schedule with 24h delay
		now := time.Now().UnixMilli()
		pendingWipe := map[string]interface{}{
			"triggeredAt": now,
			"triggeredBy": user.Name,
			"expiresAt":   now + wipeDelayMs,
		}
		wipeBytes, _ := json.Marshal(pendingWipe)
		s.DB.Set("config", "pendingWipe", wipeBytes, 0)

		internal.Log(ip, name, fmt.Sprintf("Wipe Scheduled (by %s)", user.Name), false)

		// Email all users
		s.emailAllUsers("Wipe Scheduled",
			fmt.Sprintf("A wipe has been scheduled by \"%s\". All users will be removed in 24 hours. Run \"broker wipe cancel\" to stop this.", user.Name))

		internal.Respond(w, r, http.StatusOK, map[string]string{
			"success": "Wipe scheduled. All users will be removed in 24 hours. Users have been notified via email.",
		})
	} else {
		// Execute immediately
		internal.Log(ip, name, fmt.Sprintf("Wipe Executed (by %s)", user.Name), false)
		s.executeWipe()

		internal.Respond(w, r, http.StatusOK, map[string]string{
			"success": "Wipe executed. All users have been removed.",
		})
	}
}

// POST /wipeCancel - Cancel pending wipe
func (s *Server) handleWipeCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	wipeData, _ := s.DB.Get("config", "pendingWipe")
	if wipeData == nil {
		internal.Log(ip, name, "Wipe Cancel (No Pending)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{
			"error": "No pending wipe to cancel.",
		})
		return
	}

	var pendingWipe struct {
		ExpiresAt   int64  `json:"expiresAt"`
		TriggeredBy string `json:"triggeredBy"`
	}
	json.Unmarshal(wipeData, &pendingWipe)

	if time.Now().UnixMilli() >= pendingWipe.ExpiresAt {
		internal.Log(ip, name, "Wipe Cancel (Already Expired)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{
			"error": "Pending wipe has already expired and will execute shortly.",
		})
		return
	}

	s.DB.Delete("config", "pendingWipe")
	internal.Log(ip, name, fmt.Sprintf("Wipe Cancelled (by %s)", user.Name), false)

	// Email all users
	s.emailAllUsers("Wipe Cancelled",
		fmt.Sprintf("The pending wipe has been cancelled by \"%s\".", user.Name))

	internal.Respond(w, r, http.StatusOK, map[string]string{
		"success": fmt.Sprintf("Wipe cancelled successfully. The wipe was originally scheduled by \"%s\".", pendingWipe.TriggeredBy),
	})
}

func (s *Server) executeWipe() {
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}

	for _, u := range users {
		s.DB.Delete("users", u["name"])
	}
	s.DB.Set("config", "users", []byte("[]"), 0)
}

func (s *Server) emailAllUsers(title, body string) {
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}

	for _, u := range users {
		userData, _ := s.DB.Get("users", u["name"])
		if userData != nil {
			var user model.User
			json.Unmarshal(userData, &user)
			if user.Email != "" {
				mail.Send(s.Broker, user.Email, title, body)
			}
		}
	}
}
