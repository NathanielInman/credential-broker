package model

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ScopePermission represents a user's permission level for a specific scope.
type ScopePermission struct {
	Name  string `json:"name"`
	Value string `json:"value"` // "edit" or "view"
}

// Permissions defines the access control settings for a user.
type Permissions struct {
	ViewUsers      bool              `json:"viewUsers"`
	EditUsers      bool              `json:"editUsers"`
	ViewScopeNames bool              `json:"viewScopeNames"`
	CreateScopes   bool              `json:"createScopes"`
	Scopes         []ScopePermission `json:"scopes"`
}

// User represents user data on both client and server side.
type User struct {
	// Client-side fields (stored in user.json)
	RemoteIP           string      `json:"remoteIP,omitempty"`
	Name               string      `json:"name"`
	Email              string      `json:"email"`
	LastAuthentication string      `json:"lastAuthentication,omitempty"`
	LastAction         string      `json:"lastAction,omitempty"`
	LastScope          string      `json:"lastScope,omitempty"`
	UsePassword        bool        `json:"usePassword"`
	Secret             string      `json:"secret,omitempty"` // DH shared secret
	Permissions        Permissions `json:"permissions"`

	// Server-side fields
	Date            string `json:"date,omitempty"`
	Key             string `json:"key,omitempty"`  // PGP public key
	AddedBy         string `json:"addedBy,omitempty"`
	AddedByIP       string `json:"addedByIP,omitempty"`
	TwoFactorSecret string `json:"twoFactorSecret,omitempty"`
	TwoFactorActive bool   `json:"twoFactorActive,omitempty"`
}

// HasScopeAccess checks if the user has access to a scope with the given level ("edit" or "view").
// A user with "edit" access also satisfies a "view" check.
func (u *User) HasScopeAccess(scopeName, level string) bool {
	for _, scope := range u.Permissions.Scopes {
		if scope.Name == scopeName {
			if scope.Value == level {
				return true
			}
			// "edit" permission implies "view" access
			if level == "view" && scope.Value == "edit" {
				return true
			}
			return false
		}
	}
	return false
}

// LoadUserFile reads user.json from the given directory and returns a User.
func LoadUserFile(dir string) (*User, error) {
	path := filepath.Join(dir, "user.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// SaveUserFile writes the user as user.json to the given directory.
func (u *User) SaveUserFile(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "user.json")
	return os.WriteFile(path, data, 0644)
}
