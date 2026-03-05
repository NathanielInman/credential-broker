package internal

import (
	"encoding/json"
	"fmt"
	"net/http"

	brokercrypto "github.com/nate/credential-broker/crypto"
)

// Respond encrypts the body with PGP then AES using values from request context.
func Respond(w http.ResponseWriter, r *http.Request, status int, body interface{}) {
	key, _ := r.Context().Value(CtxKey).(string)
	secret, _ := r.Context().Value(CtxSecret).(string)

	RespondWith(w, secret, key, status, body)
}

// RespondWith encrypts the body with PGP (using pubKey) then AES (using secret)
// and writes it as the HTTP response.
func RespondWith(w http.ResponseWriter, secret, pubKey string, status int, body interface{}) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	pgpEncrypted, err := brokercrypto.Encrypt(string(jsonBytes), pubKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to PGP encrypt response: %v", err), http.StatusInternalServerError)
		return
	}

	sessionEncrypted := brokercrypto.AESEncrypt(pgpEncrypted, secret)

	w.WriteHeader(status)
	fmt.Fprint(w, sessionEncrypted)
}
