package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	brokercrypto "github.com/nate/credential-broker/crypto"
	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
)

// POST /userInitialize - First-time user registration
func (s *Server) handleUserInitialize(w http.ResponseWriter, r *http.Request) {
	ip := getIP(r)

	// This route uses session encryption but not full auth
	sessionKey := r.Header.Get("key") // This is the MD5 of username
	nameHeader := r.Header.Get("name")
	emailHeader := r.Header.Get("email")

	sessionData, err := s.DB.Get("sessions", sessionKey)
	if err != nil || sessionData == nil {
		http.Error(w, "Session expired.", http.StatusUnauthorized)
		return
	}

	// The session data for userInitialize stores the secret directly
	// (it was set during authSecure using the id header)
	var sessionSecret string
	// Try to unmarshal as session object first
	var session map[string]interface{}
	if err := json.Unmarshal(sessionData, &session); err == nil {
		if s, ok := session["secret"].(string); ok {
			sessionSecret = s
		}
	}
	if sessionSecret == "" {
		// Try as raw string
		json.Unmarshal(sessionData, &sessionSecret)
	}
	if sessionSecret == "" {
		sessionSecret = string(sessionData)
	}

	name, _ := brokercrypto.AESDecrypt(nameHeader, sessionSecret)
	email, _ := brokercrypto.AESDecrypt(emailHeader, sessionSecret)

	// Parse body as JSON (not signed for this endpoint)
	body := internal.GetBody(r.Context())
	var reqBody struct {
		Name        string            `json:"name"`
		Email       string            `json:"email"`
		Key         string            `json:"key"`
		Permissions model.Permissions `json:"permissions"`
		UsePassword bool              `json:"usePassword"`
	}
	if err := json.Unmarshal([]byte(body), &reqBody); err != nil {
		http.Error(w, "Invalid request body.", http.StatusBadRequest)
		return
	}

	// Check if user already exists
	existingData, _ := s.DB.Get("users", name)
	if existingData != nil {
		var existing model.User
		json.Unmarshal(existingData, &existing)

		internal.Log(ip, name, "Initialize User", false)
		if existing.Key != reqBody.Key {
			existing.Key = reqBody.Key
			userBytes, _ := json.Marshal(existing)
			s.DB.Set("users", name, userBytes, 0)
			sendAESResponse(w, sessionSecret, map[string]string{
				"success": "Your user is properly linked. Updated public key.",
			})
		} else {
			sendAESResponse(w, sessionSecret, map[string]string{
				"success": "Your user is properly linked",
			})
		}
		return
	}

	// Get existing users and scopes
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}

	scopesData, _ := s.DB.Get("config", "scopes")
	var scopes []map[string]string
	if scopesData != nil {
		json.Unmarshal(scopesData, &scopes)
	}

	if len(scopes) == 0 && len(users) == 0 {
		// First user ever
		newUser := model.User{
			Date:        time.Now().UTC().Format(time.RFC3339),
			Name:        name,
			Email:       email,
			Key:         reqBody.Key,
			AddedBy:     "ROOT",
			AddedByIP:   ip,
			Permissions: reqBody.Permissions,
		}

		internal.Log(ip, name, "Initialize User (FIRST)", false)
		userBytes, _ := json.Marshal(newUser)
		s.DB.Set("users", name, userBytes, 0)
		usersArr, _ := json.Marshal([]map[string]string{{"name": newUser.Name}})
		s.DB.Set("config", "users", usersArr, 0)

		sendAESResponse(w, sessionSecret, map[string]string{
			"success": "First user created successfully with requested permissions.",
		})
	} else if len(users) == 0 {
		// No users but scopes exist - check strategy
		firstAccountGetsAccess := s.Broker.Strategies[3].Value

		if firstAccountGetsAccess {
			internal.Log(ip, name, "Initialize User (FIRST-GAINED-ALL-ACCESS)", false)
			scopePerms := make([]model.ScopePermission, len(scopes))
			for i, scope := range scopes {
				scopePerms[i] = model.ScopePermission{Name: scope["name"], Value: "view"}
			}

			newUser := model.User{
				Date:      time.Now().UTC().Format(time.RFC3339),
				Name:      name,
				Email:     email,
				Key:       reqBody.Key,
				AddedBy:   "ROOT",
				AddedByIP: ip,
				Permissions: model.Permissions{
					ViewUsers:      reqBody.Permissions.ViewUsers,
					EditUsers:      reqBody.Permissions.EditUsers,
					ViewScopeNames: reqBody.Permissions.ViewScopeNames,
					CreateScopes:   reqBody.Permissions.CreateScopes,
					Scopes:         scopePerms,
				},
			}

			userBytes, _ := json.Marshal(newUser)
			s.DB.Set("users", name, userBytes, 0)
			usersArr, _ := json.Marshal([]map[string]string{{"name": newUser.Name}})
			s.DB.Set("config", "users", usersArr, 0)

			scopeNames := make([]string, len(scopes))
			for i, scope := range scopes {
				scopeNames[i] = scope["name"]
			}

			sendAESResponse(w, sessionSecret, map[string]string{
				"success": fmt.Sprintf("First user created successfully and given access to scopes: %s.", joinStrings(scopeNames)),
			})
		} else {
			internal.Log(ip, name, "Initialize User (FIRST-ALL-ACCESS-DENIED)", true)
			newUser := model.User{
				Date:      time.Now().UTC().Format(time.RFC3339),
				Name:      name,
				Email:     email,
				Key:       reqBody.Key,
				AddedBy:   "ROOT",
				AddedByIP: ip,
				Permissions: model.Permissions{
					ViewUsers:      false,
					EditUsers:      false,
					ViewScopeNames: false,
					CreateScopes:   false,
					Scopes:         []model.ScopePermission{},
				},
			}

			userBytes, _ := json.Marshal(newUser)
			s.DB.Set("users", name, userBytes, 0)
			usersArr, _ := json.Marshal([]map[string]string{{"name": newUser.Name}})
			s.DB.Set("config", "users", usersArr, 0)

			sendAESResponse(w, sessionSecret, map[string]string{
				"success": "User created but without any permissions because \"First account strategy get access\" strategy turned off.",
			})
		}
	} else {
		// Users exist - need editUsers permission from admin
		var admins []string
		for _, u := range users {
			userData, _ := s.DB.Get("users", u["name"])
			if userData != nil {
				var existingUser model.User
				json.Unmarshal(userData, &existingUser)
				if existingUser.Permissions.EditUsers {
					admins = append(admins, existingUser.Name)
				}
			}
		}

		internal.Log(ip, name, "Initialize User (NOT-SETUP)", true)
		w.WriteHeader(http.StatusBadRequest)
		resp := map[string]string{
			"error": fmt.Sprintf("In order to be added as a user, please contact user administrators: %s", joinStrings(admins)),
		}
		respBytes, _ := json.Marshal(resp)
		w.Write([]byte(brokercrypto.AESEncrypt(string(respBytes), sessionSecret)))
	}
}

// POST /userAdd - Add user (requires editUsers permission)
func (s *Server) handleUserAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var newUser struct {
		Name        string            `json:"name"`
		Email       string            `json:"email"`
		Key         string            `json:"key"`
		Permissions model.Permissions `json:"permissions"`
	}

	if body == "" {
		internal.Log(ip, name, "User Add (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing user object"})
		return
	}

	if err := json.Unmarshal([]byte(body), &newUser); err != nil {
		internal.Log(ip, name, "User Add (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Invalid user object"})
		return
	}

	if !user.Permissions.EditUsers {
		internal.Log(ip, name, fmt.Sprintf("Add User (%s)", newUser.Name), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have user edit permission.", name),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Add User (%s)", newUser.Name), false)
	newUserData := model.User{
		Date:        time.Now().UTC().Format(time.RFC3339),
		Name:        newUser.Name,
		Email:       newUser.Email,
		AddedBy:     name,
		AddedByIP:   ip,
		Key:         newUser.Key,
		Permissions: newUser.Permissions,
	}

	userBytes, _ := json.Marshal(newUserData)
	s.DB.Set("users", newUserData.Name, userBytes, 0)

	// Add to users list
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}
	users = append(users, map[string]string{"name": newUserData.Name})
	usersArr, _ := json.Marshal(users)
	s.DB.Set("config", "users", usersArr, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{
		"success": fmt.Sprintf("Added user %s", newUserData.Name),
	})
}

// POST /userGet - Get specific user
func (s *Server) handleUserGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(body), &req); err != nil || req.Name == "" {
		internal.Log(ip, name, "User Get (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing requestedUsername"})
		return
	}

	// User can always view themselves
	if req.Name == name {
		internal.Log(ip, name, fmt.Sprintf("User Get (%s)", req.Name), false)
		internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": user})
		return
	}

	if !user.Permissions.EditUsers {
		internal.Log(ip, name, fmt.Sprintf("User Get (%s)", req.Name), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have user edit permission.", name),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("User Get (%s)", req.Name), false)
	userData, err := s.DB.Get("users", req.Name)
	if err != nil || userData == nil {
		internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": nil})
		return
	}

	var requestedUser model.User
	json.Unmarshal(userData, &requestedUser)
	internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": requestedUser})
}

// POST /userGetAll - List all users
func (s *Server) handleUserGetAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	if !user.Permissions.ViewUsers {
		internal.Log(ip, name, "Get All Users", true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have user edit permission.", name),
		})
		return
	}

	internal.Log(ip, name, "Get All Users", false)
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}

	names := make([]string, len(users))
	for i, u := range users {
		names[i] = u["name"]
	}

	internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": names})
}

// POST /userModify - Update user
func (s *Server) handleUserModify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	if body == "" {
		internal.Log(ip, name, "User Modify (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing requestedUsername"})
		return
	}

	var reqBody struct {
		Name        string            `json:"name"`
		Email       string            `json:"email"`
		Key         string            `json:"key"`
		Permissions model.Permissions `json:"permissions"`
	}
	json.Unmarshal([]byte(body), &reqBody)

	if !user.Permissions.EditUsers {
		internal.Log(ip, name, fmt.Sprintf("User Modify (%s)", reqBody.Name), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have user edit permission.", name),
		})
		return
	}

	targetData, _ := s.DB.Get("users", reqBody.Name)
	if targetData == nil {
		internal.Log(ip, name, fmt.Sprintf("User Modify (%s-NO-USER)", reqBody.Name), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not exist.", name),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("User Modify (%s)", reqBody.Name), false)
	modifiedUser := model.User{
		Date:        time.Now().UTC().Format(time.RFC3339),
		Name:        reqBody.Name,
		Email:       reqBody.Email,
		AddedBy:     name,
		AddedByIP:   ip,
		Key:         reqBody.Key,
		Permissions: reqBody.Permissions,
	}

	userBytes, _ := json.Marshal(modifiedUser)
	s.DB.Set("users", reqBody.Name, userBytes, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Modified user successfully."})
}

// POST /userDelete - Remove user
func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		Target string `json:"target"`
	}

	if err := json.Unmarshal([]byte(body), &reqBody); err != nil || reqBody.Target == "" {
		internal.Log(ip, name, "User Delete (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Request has been tempered with!"})
		return
	}

	if !user.Permissions.EditUsers {
		internal.Log(ip, name, fmt.Sprintf("User Delete (%s)", reqBody.Target), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have user edit permission.", reqBody.Target),
		})
		return
	}

	targetData, _ := s.DB.Get("users", reqBody.Target)
	if targetData == nil {
		internal.Log(ip, name, fmt.Sprintf("User Delete (%s-NO-USER)", reqBody.Target), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not exist.", reqBody.Target),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("User Delete (%s)", reqBody.Target), false)

	// Remove from users list
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}
	var filtered []map[string]string
	for _, u := range users {
		if u["name"] != reqBody.Target {
			filtered = append(filtered, u)
		}
	}
	usersArr, _ := json.Marshal(filtered)
	s.DB.Set("config", "users", usersArr, 0)
	s.DB.Delete("users", reqBody.Target)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Deleted user successfully."})
}

func sendAESResponse(w http.ResponseWriter, secret string, body interface{}) {
	respBytes, _ := json.Marshal(body)
	encrypted := brokercrypto.AESEncrypt(string(respBytes), secret)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(encrypted))
}

func joinStrings(s []string) string {
	result := ""
	for i, v := range s {
		if i > 0 {
			result += ","
		}
		result += v
	}
	return result
}
