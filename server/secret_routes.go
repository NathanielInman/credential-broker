package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nate/credential-broker/internal"
	"github.com/nate/credential-broker/model"
)

// POST /secretAdd - Add secret to scope
func (s *Server) handleSecretAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName   string `json:"scopeName"`
		SecretName  string `json:"secretName"`
		SecretValue string `json:"secretValue"`
	}

	if err := json.Unmarshal([]byte(body), &reqBody); err != nil || reqBody.ScopeName == "" || reqBody.SecretName == "" || reqBody.SecretValue == "" {
		internal.Log(ip, name, "Secret Add (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopeName, secretName or secretValue."})
		return
	}

	hasScopeEditAccess := user.HasScopeAccess(reqBody.ScopeName, "edit")
	if !hasScopeEditAccess {
		internal.Log(ip, name, fmt.Sprintf("Secret Add (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope edit permission.", name),
		})
		return
	}

	// Check scope exists
	scopeData, _ := s.DB.Get("scopes", reqBody.ScopeName)
	if scopeData == nil {
		internal.Log(ip, name, fmt.Sprintf("Secret Add (%s-NO-SCOPE)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Scope \"%s\" does not exist.", reqBody.ScopeName),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Secret Add (%s)", reqBody.ScopeName), false)

	var secrets map[string]string
	if err := json.Unmarshal(scopeData, &secrets); err != nil || secrets == nil {
		secrets = make(map[string]string)
	}

	secrets[reqBody.SecretName] = reqBody.SecretValue
	secretsBytes, _ := json.Marshal(secrets)
	s.DB.Set("scopes", reqBody.ScopeName, secretsBytes, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Added secret successfully."})
}

// POST /secretModify - Update secret in scope
func (s *Server) handleSecretModify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName   string `json:"scopeName"`
		SecretName  string `json:"secretName"`
		SecretValue string `json:"secretValue"`
	}

	if err := json.Unmarshal([]byte(body), &reqBody); err != nil || reqBody.ScopeName == "" || reqBody.SecretName == "" || reqBody.SecretValue == "" {
		internal.Log(ip, name, "Secret Modify (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopeName, secretName or secretValue"})
		return
	}

	hasScopeEditAccess := user.HasScopeAccess(reqBody.ScopeName, "edit")
	if !hasScopeEditAccess {
		internal.Log(ip, name, fmt.Sprintf("Secret Modify (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope edit permission.", name),
		})
		return
	}

	scopeData, _ := s.DB.Get("scopes", reqBody.ScopeName)
	if scopeData == nil {
		internal.Log(ip, name, fmt.Sprintf("Secret Modify (%s-NO-SCOPE)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Scope \"%s\" does not exist.", reqBody.ScopeName),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Secret Modify (%s)", reqBody.ScopeName), false)

	var secrets map[string]string
	if err := json.Unmarshal(scopeData, &secrets); err != nil || secrets == nil {
		secrets = make(map[string]string)
	}

	secrets[reqBody.SecretName] = reqBody.SecretValue
	secretsBytes, _ := json.Marshal(secrets)
	s.DB.Set("scopes", reqBody.ScopeName, secretsBytes, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Modified secret successfully."})
}

// POST /secretDelete - Remove secret from scope
func (s *Server) handleSecretDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := ctx.Value(internal.CtxName).(string)
	user := ctx.Value(internal.CtxUser).(*model.User)
	ip := getIP(r)

	body := internal.GetBody(ctx)
	var reqBody struct {
		ScopeName  string `json:"scopeName"`
		SecretName string `json:"secretName"`
	}

	if err := json.Unmarshal([]byte(body), &reqBody); err != nil || reqBody.ScopeName == "" || reqBody.SecretName == "" {
		internal.Log(ip, name, "Secret Delete (Bad Request)", true)
		internal.Respond(w, r, http.StatusBadRequest, map[string]string{"error": "Missing scopeName or secretName"})
		return
	}

	hasScopeEditAccess := user.HasScopeAccess(reqBody.ScopeName, "edit")
	if !hasScopeEditAccess {
		internal.Log(ip, name, fmt.Sprintf("Secret Delete (%s)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("User \"%s\" does not have scope edit permission.", name),
		})
		return
	}

	scopeData, _ := s.DB.Get("scopes", reqBody.ScopeName)
	if scopeData == nil {
		internal.Log(ip, name, fmt.Sprintf("Secret Delete (%s-NO-SCOPE)", reqBody.ScopeName), true)
		internal.Respond(w, r, http.StatusUnauthorized, map[string]string{
			"error": fmt.Sprintf("Scope \"%s\" does not exist.", reqBody.ScopeName),
		})
		return
	}

	internal.Log(ip, name, fmt.Sprintf("Secret Delete (%s)", reqBody.ScopeName), false)

	var secrets map[string]string
	json.Unmarshal(scopeData, &secrets)
	delete(secrets, reqBody.SecretName)
	secretsBytes, _ := json.Marshal(secrets)
	s.DB.Set("scopes", reqBody.ScopeName, secretsBytes, 0)

	internal.Respond(w, r, http.StatusOK, map[string]string{"success": "Deleted scope secret successfully."})
}
