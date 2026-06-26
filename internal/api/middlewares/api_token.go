package middlewares

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
	authentication_storage "github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

type apiTokenKey string

const (
	APITokenContextKey apiTokenKey = "api_token"
	apiTokenPrefix                 = "hht_"
	bearerPrefix                   = "Bearer "
)

func APITokenAuthMiddleware(apiTokenRepository authentication_storage.APITokenRepository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenValue := strings.TrimSpace(r.Header.Get("X-API-Token"))
		if tokenValue == "" {
			tokenValue = extractBearerToken(r.Header.Get("Authorization"))
		}
		if tokenValue == "" {
			http.Error(w, "api token is required", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(tokenValue, apiTokenPrefix) {
			http.Error(w, "invalid api token", http.StatusUnauthorized)
			return
		}
		rawToken := tokenValue[len(apiTokenPrefix):]
		if len(rawToken) == 0 {
			http.Error(w, "invalid api token", http.StatusUnauthorized)
			return
		}

		hashedToken := hashToken(tokenValue)
		apiToken, err := apiTokenRepository.GetAPITokenByHash(r.Context(), hashedToken)
		if err != nil {
			http.Error(w, "invalid api token", http.StatusUnauthorized)
			return
		}
		if apiToken.RevokedAt != nil {
			http.Error(w, "api token is revoked", http.StatusUnauthorized)
			return
		}
		if apiToken.ExpiresAt != nil && apiToken.ExpiresAt.Before(time.Now()) {
			http.Error(w, "api token is expired", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), APITokenContextKey, apiToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetAPITokenFromContext(ctx context.Context) (authentication.APIToken, error) {
	token, ok := ctx.Value(APITokenContextKey).(authentication.APIToken)
	if !ok {
		return authentication.APIToken{}, errors.New("api token not found in context")
	}
	return token, nil
}

func extractBearerToken(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if !strings.HasPrefix(headerValue, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(headerValue, bearerPrefix))
}

func hashToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
