package middlewares

import (
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/web/i18n"
	"github.com/yazmeyaa/hosthalla/tokibundle"
	"golang.org/x/text/language"
)

const localeCookieName = "locale"

func WithTokiReaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader := resolveTokiReader(r)

		ctx := i18n.WithReader(r.Context(), reader)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func resolveTokiReader(r *http.Request) tokibundle.Reader {
	if cookie, err := r.Cookie(localeCookieName); err == nil {
		if tag, err := language.Parse(cookie.Value); err == nil {
			if reader, confidence := tokibundle.Match(tag); confidence != language.No {
				return reader
			}
		}
	}

	if tags, _, err := language.ParseAcceptLanguage(
		r.Header.Get("Accept-Language"),
	); err == nil && len(tags) > 0 {
		reader, _ := tokibundle.Match(tags...)
		return reader
	}

	return tokibundle.Default()
}
