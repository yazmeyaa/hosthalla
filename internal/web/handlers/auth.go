package handlers

import (
	"net/http"

	"github.com/yazmeyaa/hosthalla/ui/pages/auth_page"
)

type AuthHandler struct {
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

func (h *AuthHandler) Auth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth_page.AuthPage().Render(ctx, w)
}
