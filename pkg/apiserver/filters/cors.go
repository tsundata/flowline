package filters

import (
	"github.com/tsundata/flowline/pkg/util/flog"
	"net/http"
	"regexp"
	"strings"
)

func WithCORS(handler http.Handler, allowedOriginPatterns []string, allowedMethods []string, allowedHeaders []string, exposedHeaders []string, allowCredentials string) http.Handler {
	if len(allowedOriginPatterns) == 0 {
		return handler
	}
	allowedOriginPatternsREs := allowedOriginRegexps(allowedOriginPatterns)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		origin := req.Header.Get("Origin")
		if origin != "" {
			allowed := false
			for _, re := range allowedOriginPatternsREs {
				if allowed = re.MatchString(origin); allowed {
					break
				}
			}
			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Set defaults for methods and headers if nothing was passed
				if allowedMethods == nil {
					allowedMethods = []string{"POST", "GET", "OPTIONS", "PUT", "DELETE", "PATCH"}
				}
				if allowedHeaders == nil {
					allowedHeaders = []string{"Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "X-Requested-With", "If-Modified-Since"}
				}
				if exposedHeaders == nil {
					exposedHeaders = []string{"Date"}
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(allowedHeaders, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(exposedHeaders, ", "))
				w.Header().Set("Access-Control-Allow-Credentials", allowCredentials)

				// Stop here if its a preflight OPTIONS request
				if req.Method == "OPTIONS" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}
		// Dispatch to the next handler
		handler.ServeHTTP(w, req)
	})
}

func allowedOriginRegexps(allowedOrigins []string) []*regexp.Regexp {
	res, err := compileRegexps(allowedOrigins)
	if err != nil {
		flog.Errorf("Invalid CORS allowed origin, --cors-allowed-origins flag was set to %v - %v", strings.Join(allowedOrigins, ","), err)
	}
	return res
}

// Takes a list of strings and compiles them into a list of regular expressions
func compileRegexps(regexpStrings []string) ([]*regexp.Regexp, error) {
	var regexps []*regexp.Regexp
	for _, regexpStr := range regexpStrings {
		r, err := regexp.Compile(regexpStr)
		if err != nil {
			return []*regexp.Regexp{}, err
		}
		regexps = append(regexps, r)
	}
	return regexps, nil
}
