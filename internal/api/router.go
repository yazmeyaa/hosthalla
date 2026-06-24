package api

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	return mux
}
