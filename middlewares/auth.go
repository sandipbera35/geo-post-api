package middlewares

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// publicKey is loaded once at boot via LoadPublicKey and read (never
// mutated) by every request afterwards, so no locking is needed.
var publicKey *rsa.PublicKey

// LoadPublicKey reads the RSA public key PEM file at path and stores it
// package-wide for RequireAuth to verify tokens against. It must be called
// once during startup, before the server begins handling requests — this is
// the Zero-Trust boundary: no auth-service call happens per request.
func LoadPublicKey(path string) error {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read jwt public key file: %w", err)
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM(pemBytes)
	if err != nil {
		return fmt.Errorf("parse jwt public key: %w", err)
	}

	publicKey = key
	return nil
}

// RequireAuth verifies a Bearer JWT against the loaded RSA public key and,
// on success, sets "user_id" (the token's sub claim) in the Gin context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed authorization header"})
			return
		}
		tokenString := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed authorization header"})
			return
		}

		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return publicKey, nil
		}, jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodRS384.Alg(),
			jwt.SigningMethodRS512.Alg(),
		}))
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if claims.Subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token missing sub claim"})
			return
		}

		c.Set("user_id", claims.Subject)
		c.Next()
	}
}
