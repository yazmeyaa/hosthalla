package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yazmeyaa/hosthalla/internal/web/i18n"
	"github.com/yazmeyaa/hosthalla/tokibundle"
	"golang.org/x/text/language"
)

func TestResolveTokiReader(t *testing.T) {
	tests := []struct {
		name           string
		cookie         string
		acceptLanguage string
		wantLocale     string
	}{
		{
			name:   "english cookie overrides russian header",
			cookie: "locale=en", acceptLanguage: "ru", wantLocale: "en",
		},
		{
			name:   "russian cookie overrides english header",
			cookie: "locale=ru", acceptLanguage: "en", wantLocale: "ru",
		},
		{
			name:   "regional english cookie",
			cookie: "locale=en-US", acceptLanguage: "ru", wantLocale: "en",
		},
		{
			name:   "regional russian cookie",
			cookie: "locale=ru-RU", acceptLanguage: "en", wantLocale: "ru",
		},
		{
			name:   "malformed cookie locale falls back to header",
			cookie: "locale=en-@", acceptLanguage: "ru", wantLocale: "ru",
		},
		{
			name:   "malformed cookie syntax falls back to header",
			cookie: `locale="ru`, acceptLanguage: "en", wantLocale: "en",
		},
		{
			name:   "unsupported cookie falls back to header",
			cookie: "locale=ja", acceptLanguage: "ru", wantLocale: "ru",
		},
		{
			name:   "empty cookie falls back to header",
			cookie: "locale=", acceptLanguage: "ru", wantLocale: "ru",
		},
		{
			name:   "malformed cookie without header uses default",
			cookie: "locale=en-@", wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:   "unsupported cookie without header uses default",
			cookie: "locale=ja", wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:   "empty cookie without header uses default",
			cookie: "locale=", wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:           "regional english header",
			acceptLanguage: "en-GB", wantLocale: "en",
		},
		{
			name:           "regional russian header",
			acceptLanguage: "ru-RU", wantLocale: "ru",
		},
		{
			name:           "higher russian weight wins over header order",
			acceptLanguage: "en;q=0.2, ru;q=0.9", wantLocale: "ru",
		},
		{
			name:           "higher english weight wins over header order",
			acceptLanguage: "ru;q=0.2, en;q=0.9", wantLocale: "en",
		},
		{
			name:           "implicit weight is one",
			acceptLanguage: "en;q=0.9, ru", wantLocale: "ru",
		},
		{
			name:           "unsupported first preference falls back to supported language",
			acceptLanguage: "ja;q=1, ru;q=0.8, en;q=0.2", wantLocale: "ru",
		},
		{
			name:           "zero weight is excluded",
			acceptLanguage: "ru;q=0, en;q=0.5", wantLocale: "en",
		},
		{
			name:       "absent header uses default",
			wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:           "invalid header uses default",
			acceptLanguage: "@invalid", wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:           "invalid weight uses default",
			acceptLanguage: "ru;q=invalid", wantLocale: tokibundle.DefaultLocale,
		},
		{
			name:           "unsupported header uses default",
			acceptLanguage: "ja", wantLocale: tokibundle.DefaultLocale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != "" {
				request.Header.Set("Cookie", tt.cookie)
			}
			if tt.acceptLanguage != "" {
				request.Header.Set("Accept-Language", tt.acceptLanguage)
			}

			if got := resolveTokiReader(request).Locale().String(); got != tt.wantLocale {
				t.Fatalf("unexpected locale: got %q, want %q", got, tt.wantLocale)
			}
		})
	}
}

func TestWithTokiReaderMiddlewareContext(t *testing.T) {
	type contextKey struct{}

	parentReader, _ := tokibundle.Match(language.Russian)
	parent := context.WithValue(context.Background(), contextKey{}, "request value")
	parent = i18n.WithReader(parent, parentReader)
	parent, cancel := context.WithCancel(parent)
	defer cancel()

	var received context.Context
	handler := WithTokiReaderMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Context()
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name           string
		cookie         string
		acceptLanguage string
		wantLocale     string
	}{
		{name: "russian cookie", cookie: "ru", acceptLanguage: "en", wantLocale: "ru"},
		{name: "english cookie", cookie: "en", acceptLanguage: "ru", wantLocale: "en"},
		{name: "russian header", acceptLanguage: "ru", wantLocale: "ru"},
		{name: "default after russian request", wantLocale: tokibundle.DefaultLocale},
	}
	contexts := make(map[string]context.Context, len(tests))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/dashboard/subscribe", nil).WithContext(parent)
			if tt.cookie != "" {
				request.AddCookie(&http.Cookie{Name: localeCookieName, Value: tt.cookie})
			}
			if tt.acceptLanguage != "" {
				request.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			response := httptest.NewRecorder()
			received = nil

			handler.ServeHTTP(response, request)

			if received == nil {
				t.Fatal("downstream handler was not called")
			}
			contexts[tt.name] = received
			if response.Code != http.StatusNoContent {
				t.Errorf("unexpected status: got %d, want %d", response.Code, http.StatusNoContent)
			}
			if got := i18n.ReaderFromContext(received).Locale().String(); got != tt.wantLocale {
				t.Errorf("unexpected context reader locale: got %q, want %q", got, tt.wantLocale)
			}
			if got := i18n.Reader(received).Locale().String(); got != tt.wantLocale {
				t.Errorf("unexpected reader helper locale: got %q, want %q", got, tt.wantLocale)
			}
			if got := received.Value(contextKey{}); got != "request value" {
				t.Errorf("parent context value was not propagated: got %v", got)
			}
			if request.Context() != parent {
				t.Error("original request context was replaced")
			}
			if got := i18n.ReaderFromContext(request.Context()).Locale().String(); got != "ru" {
				t.Errorf("original request reader was changed: got %q, want ru", got)
			}
		})
	}

	cancel()
	for _, tt := range tests {
		ctx, ok := contexts[tt.name]
		if !ok {
			continue
		}
		if got := i18n.ReaderFromContext(ctx).Locale().String(); got != tt.wantLocale {
			t.Errorf("%s: later requests changed reader: got %q, want %q", tt.name, got, tt.wantLocale)
		}
		if ctx.Err() != context.Canceled {
			t.Errorf("%s: parent cancellation was not propagated: got %v", tt.name, ctx.Err())
		}
	}
}
