package handlers

import "net/http"

func isHTMXBoostedNavigationRequest(r *http.Request) bool {
	return r.Method == http.MethodGet &&
		r.Header.Get("HX-Request") == "true" &&
		r.Header.Get("HX-Boosted") == "true"
}
