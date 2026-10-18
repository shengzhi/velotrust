package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAgeSeconds  int
}

// DefaultCORSConfig supports browser clients out of the box. Production
// deployments should replace "*" with an explicit origin allow-list.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "Accept-Language", "X-Requested-With"},
		MaxAgeSeconds:  600}
}

// CORS handles browser preflight requests before JRPC request parsing.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	defaults := DefaultCORSConfig()
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = defaults.AllowedOrigins
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = defaults.AllowedMethods
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = defaults.AllowedHeaders
	}
	if config.MaxAgeSeconds <= 0 {
		config.MaxAgeSeconds = defaults.MaxAgeSeconds
	}
	allowed := make(map[string]struct{}, len(config.AllowedOrigins))
	for _, origin := range config.AllowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	methods, headers := strings.Join(config.AllowedMethods, ", "), strings.Join(config.AllowedHeaders, ", ")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allows(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAgeSeconds))
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func allows(origins map[string]struct{}, origin string) bool {
	if _, ok := origins["*"]; ok {
		return true
	}
	_, ok := origins[origin]
	return ok
}
