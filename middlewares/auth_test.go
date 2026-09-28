package middlewares

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func writeTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	path := filepath.Join(t.TempDir(), "pub.pem")
	if err := os.WriteFile(path, pubPEM, 0o600); err != nil {
		t.Fatalf("write pem: %v", err)
	}

	return priv, path
}

func sign(t *testing.T, priv *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(priv)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return tok
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	return r
}

func TestRequireAuth_ValidToken(t *testing.T) {
	priv, pubPath := writeTestKeyPair(t)
	if err := LoadPublicKey(pubPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	token := sign(t, priv, jwt.RegisteredClaims{
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "user-123") {
		t.Fatalf("expected body to contain user-123, got %s", w.Body.String())
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	_, pubPath := writeTestKeyPair(t)
	if err := LoadPublicKey(pubPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_WrongKey(t *testing.T) {
	_, pubPath := writeTestKeyPair(t)
	if err := LoadPublicKey(pubPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	otherPriv, _ := writeTestKeyPair(t)
	token := sign(t, otherPriv, jwt.RegisteredClaims{
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for signature from wrong key, got %d", w.Code)
	}
}

func TestRequireAuth_RejectsNoneAlg(t *testing.T) {
	_, pubPath := writeTestKeyPair(t)
	if err := LoadPublicKey(pubPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	// alg=none attack: unsigned token asserting an arbitrary subject.
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Subject: "attacker",
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+unsigned)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for alg=none token, got %d", w.Code)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	priv, pubPath := writeTestKeyPair(t)
	if err := LoadPublicKey(pubPath); err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}

	token := sign(t, priv, jwt.RegisteredClaims{
		Subject:   "user-123",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	setupRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", w.Code)
	}
}
