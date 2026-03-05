package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
)

// POST /scopeAdd - Create scope
func (s *Server) handleScopeAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName     string `json:"scopeName"`
		ScopePublicKey string `json:"scopePublicKey"`
	}
	json.Unmarshal([]byte(body), &reqBody)

	if reqBody.ScopeName == "" {
		internal.Log(ip, name, "Add Scope (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopePublicKey or scopeName"})
		return
	}

	if !user.Permissions.CreateScopes {
		internal.Log(ip, name, fmt.Sprintf("Add Scope (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope create permission.", name),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Add Scope (%s)", reqBody.ScopeName), false)

	// Check if scope already exists
	scopesData, _ := s.DB.Get("config", "scopes")
	var scopes []map[string]string
	if scopesData != nil {
		json.Unmarshal(scopesData, &scopes)
	}

	for _, scope := range scopes {
		if scope["name"] == reqBody.ScopeName {
			internal.Log(ip, name, fmt.Sprintf("Add Scope (%s)", reqBody.ScopeName), true)
			internal.Respond(w, r, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("Scope name \"%s\" already exists and cannot be created.", reqBody.ScopeName),
			})
			return
		}
	}

	// Create empty scope
	s.DB.Set("scopes", reqBody.ScopeName, []byte("{}"), 0)

	// Add to scopes list
	scopes = append(scopes, map[string]string{"name": reqBody.ScopeName, "publicKey": reqBody.ScopePublicKey})
	scopesArr, _ := json.Marshal(scopes)
	s.DB.Set("config", "scopes", scopesArr, 0)

	// Add scope to requesting user's permissions
	user.Permissions.Scopes = append(user.Permissions.Scopes, model.ScopePermission{
		Name:  reqBody.ScopeName,
		Value: "edit",
	})
	userBytes, _ := json.Marshal(user)
	s.DB.Set("users", name, userBytes, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{
		"success": fmt.Sprintf("Added scope %s", reqBody.ScopeName),
	})
}

// POST /scopeGet - Retrieve scope secrets
func (s *Server) handleScopeGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName string `json:"scopeName"`
	}
	json.Unmarshal([]byte(body), &reqBody)

	if reqBody.ScopeName == "" {
		internal.Log(ip, name, "Get Scope (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopeName"})
		return
	}

	hasScopeAccess := user.HasScopeAccess(reqBody.ScopeName, "view") || user.HasScopeAccess(reqBody.ScopeName, "edit")

	if !hasScopeAccess {
		internal.Log(ip, name, fmt.Sprintf("Get Scope (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have get all scopes permission.", name),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Get Scope (%s)", reqBody.ScopeName), false)
	scopeData, err := s.DB.Get("scopes", reqBody.ScopeName)
	if err != nil || scopeData == nil {
		internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": map[string]string{}})
		return
	}

	var scopeSecrets map[string]string
	json.Unmarshal(scopeData, &scopeSecrets)
	internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": scopeSecrets})
}

// POST /scopeGetAll - List all scope names
func (s *Server) handleScopeGetAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	if !user.Permissions.ViewScopeNames {
		internal.Log(ip, name, "Get All Scope Names (Bad Request)", true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have get all scope names permission.", user.Name),
		})
		return
	}

	internal.Log(ip, name, "Get All Scope Names", false)
	scopesData, _ := s.DB.Get("config", "scopes")
	var scopes []map[string]string
	if scopesData != nil {
		json.Unmarshal(scopesData, &scopes)
	}

	names := make([]string, len(scopes))
	for i, scope := range scopes {
		names[i] = scope["name"]
	}

	internal.Respond(w, r, http.StatusOK, map[string]interface{}{"success": names})
}

// POST /scopeModify - Rename scope
func (s *Server) handleScopeModify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		Target         string `json:"target"`
		ScopeName      string `json:"scopeName"`
		ScopePublicKey string `json:"scopePublicKey"`
	}
	json.Unmarshal([]byte(body), &reqBody)

	if reqBody.Target == "" || reqBody.ScopeName == "" {
		internal.Log(ip, name, "Scope Modify (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing target, scopeName or scopePublicKey"})
		return
	}

	hasScopeEditAccess := user.HasScopeAccess(reqBody.ScopeName, "edit")
	if !hasScopeEditAccess {
		internal.Log(ip, name, fmt.Sprintf("Scope Modify (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope edit permission.", name),
		})
		return
	}

	targetData, _ := s.DB.Get("scopes", reqBody.Target)
	if targetData == nil {
		internal.Log(ip, name, fmt.Sprintf("Scope Modify (%s-NO-SCOPE)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Scope \"%s\" does not exist.", reqBody.ScopeName),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Scope Modify (%s->%s)", reqBody.Target, reqBody.ScopeName), false)

	if reqBody.Target != reqBody.ScopeName {
		// Rename: move data, update all users
		s.DB.Delete("scopes", reqBody.Target)
		s.DB.Set("scopes", reqBody.ScopeName, targetData, 0)

		// Update users' scope references
		usersData, _ := s.DB.Get("config", "users")
		var users []map[string]string
		if usersData != nil {
			json.Unmarshal(usersData, &users)
		}

		for _, u := range users {
			userData, _ := s.DB.Get("users", u["name"])
			if userData != nil {
				var userObj model.User
				json.Unmarshal(userData, &userObj)
				for i, scope := range userObj.Permissions.Scopes {
					if scope.Name == reqBody.Target {
						userObj.Permissions.Scopes[i].Name = reqBody.ScopeName
						userBytes, _ := json.Marshal(userObj)
						s.DB.Set("users", u["name"], userBytes, 0)
						break
					}
				}
			}
		}
	}

	// Update scopes list
	scopesData, _ := s.DB.Get("config", "scopes")
	var scopes []map[string]string
	if scopesData != nil {
		json.Unmarshal(scopesData, &scopes)
	}
	for i, scope := range scopes {
		if scope["name"] == reqBody.Target {
			scopes[i]["name"] = reqBody.ScopeName
			scopes[i]["publicKey"] = reqBody.ScopePublicKey
			break
		}
	}
	scopesArr, _ := json.Marshal(scopes)
	s.DB.Set("config", "scopes", scopesArr, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Modified scope successfully."})
}

// POST /scopeDelete - Remove scope
func (s *Server) handleScopeDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName string `json:"scopeName"`
	}
	json.Unmarshal([]byte(body), &reqBody)

	if reqBody.ScopeName == "" {
		internal.Log(ip, name, "Scope Delete (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopeName"})
		return
	}

	hasScopeEditAccess := user.HasScopeAccess(reqBody.ScopeName, "edit")
	if !hasScopeEditAccess {
		internal.Log(ip, name, fmt.Sprintf("Scope Delete (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope edit permission", name),
		})
		return
	}

	targetData, _ := s.DB.Get("scopes", reqBody.ScopeName)
	if targetData == nil {
		internal.Log(ip, name, fmt.Sprintf("Scope Delete (%s-NO_SCOPE)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Scope \"%s\" does not exist.", reqBody.ScopeName),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Scope Delete (%s)", reqBody.ScopeName), false)

	// Remove scope from all users
	usersData, _ := s.DB.Get("config", "users")
	var users []map[string]string
	if usersData != nil {
		json.Unmarshal(usersData, &users)
	}

	for _, u := range users {
		userData, _ := s.DB.Get("users", u["name"])
		if userData != nil {
			var userObj model.User
			json.Unmarshal(userData, &userObj)
			var filtered []model.ScopePermission
			for _, scope := range userObj.Permissions.Scopes {
				if scope.Name != reqBody.ScopeName {
					filtered = append(filtered, scope)
				}
			}
			userObj.Permissions.Scopes = filtered
			userBytes, _ := json.Marshal(userObj)
			s.DB.Set("users", u["name"], userBytes, 0)
		}
	}

	// Delete scope data
	s.DB.Delete("scopes", reqBody.ScopeName)

	// Remove from scopes list
	scopesData, _ := s.DB.Get("config", "scopes")
	var scopes []map[string]string
	if scopesData != nil {
		json.Unmarshal(scopesData, &scopes)
	}
	var filteredScopes []map[string]string
	for _, scope := range scopes {
		if scope["name"] != reqBody.ScopeName {
			filteredScopes = append(filteredScopes, scope)
		}
	}
	scopesArr, _ := json.Marshal(filteredScopes)
	s.DB.Set("config", "scopes", scopesArr, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Deleted scope successfully."})
}
