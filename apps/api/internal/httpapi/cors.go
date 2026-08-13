package httpapi

import "net/http"

const (
	corsAllowMethods     = "GET, POST, PATCH, OPTIONS"
	corsAllowHeaders     = "Content-Type"
	corsAllowCredentials = "true"
)

// CORS returns middleware that applies single-origin CORS configuration to the
// wrapped handler. It stays separate from the identity middleware so preflight
// requests (which deliberately carry no tenant headers) never hit auth checks.
//
// The response is attributed to the configured origin only when the request's
// Origin header matches it exactly; mismatched or absent origins get no CORS
// headers and the browser blocks the call.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", corsAllowCredentials)
			}

			if r.Method == http.MethodOptions {
				if origin != "" && origin == allowedOrigin {
					w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
					w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
