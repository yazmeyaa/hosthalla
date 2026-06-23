package middlewares

import (
	"context"
	"errors"
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/authentication"
	"github.com/yazmeyaa/hosthalla/internal/authentication/storage"
)

type sessionKey string

const SessionKey sessionKey = "session"

func AuthMiddleware(sessionStore storage.SessionRepository, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			return
		}
		sessionID := cookie.Value
		session, err := sessionStore.GetSessionByID(r.Context(), sessionID)
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusSeeOther)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), SessionKey, session))
		next.ServeHTTP(w, r)
	})
}

func GetSessionFromContext(ctx context.Context) (authentication.Session, error) {
	session, ok := ctx.Value(SessionKey).(authentication.Session)
	if !ok {
		return authentication.Session{}, errors.New("session not found")
	}
	return session, nil
}
