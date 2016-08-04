package streamer

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}

type handshakeToken struct {
	AppID  string `json:"app_id"`
	UserID string `json:"user_id"`
}

func decryptToken(tkn string) (decryptedTkn *handshakeToken, err error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(tkn)
	if err != nil {
		return decryptedTkn, err
	}

	err = json.Unmarshal(decodedBytes, &decryptedTkn)
	if err != nil {
		return decryptedTkn, err
	}
	return decryptedTkn, err
}

func verifyToken(token string, signature string) bool {

	if os.Getenv("SIPHON_ENV") == "testing" {
		return true
	}

	b, err := ioutil.ReadFile("/code/.keys/handshake/handshake.pub")
	if err != nil {
		log.Printf("Problem opening handshake key file: %v", err)
		return false
	}

	block, _ := pem.Decode(b)
	re, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Printf("Problem parsing handshake key file: %v", err)
		return false
	}
	key := re.(*rsa.PublicKey)

	h := sha256.New()
	s, _ := base64.StdEncoding.DecodeString(token)
	h.Write(s)
	hashed := h.Sum(nil)

	err = rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed, []byte(signature))
	if err != nil {
		log.Printf("VerifyPKCS1v15() error: %v", err)
		return false
	}
	return true // if we got this far, the signature is valid.
}

func authorizedRequest(r *http.Request) (tkn *handshakeToken, connType string, err error) {
	connType, err = getValFromRequest("type", r)
	if err != nil {
		return nil, "", err
	}

	appID, err := getValFromRequest("app_id", r)
	if err != nil {
		return nil, "", err
	}

	token, err := getValFromRequest("handshake_token", r)
	signature, err := getValFromRequest("handshake_signature", r)
	obj, err := decryptToken(token)
	if err != nil || !verifyToken(token, signature) {
		return nil, "", err
	}

	if obj.AppID != appID {
		return nil, "", errors.New("Bad handshake_token/app_id pair")
	}
	return obj, connType, err
}
