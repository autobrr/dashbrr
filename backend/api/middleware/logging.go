package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	// Enable console writer with colors
	output := zerolog.ConsoleWriter{
		Out:     os.Stdout,
		NoColor: false,
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}

// Logger returns a gin middleware for logging HTTP requests with zerolog
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		//start := time.Now()

		// Process request
		c.Next()

		// Get the query string and redact sensitive information
		query := c.Request.URL.RawQuery
		if query != "" {
			parsedURL, err := url.ParseQuery(query)
			if err == nil {
				// List of parameter names to redact
				sensitiveParams := []string{
					"apiKey",
					"api_key",
					"key",
					"token",
					"password",
					"secret",
				}

				// Redact sensitive parameters
				for param := range parsedURL {
					for _, sensitive := range sensitiveParams {
						if strings.Contains(strings.ToLower(param), strings.ToLower(sensitive)) {
							parsedURL.Set(param, "[REDACTED]")
						}
					}
				}
				query = parsedURL.Encode()
			}
		}

		// Create the path with redacted query string
		path := c.Request.URL.Path
		if query != "" {
			path = path + "?" + query
		}

		// Get error if exists
		//var err error
		//if len(c.Errors) > 0 {
		//	err = c.Errors.Last()
		//}

		// Log the request with zerolog
		//event := log.Info()
		//if err != nil {
		//	event = log.Error().Err(err)
		//}

		//event.
		//	Str("method", c.Request.Method).
		//	Str("path", path).
		//	Int("status", c.Writer.Status()).
		//	Dur("latency", time.Since(start)).
		//	Str("ip", c.ClientIP()).
		//	Int("bytes", c.Writer.Size()).
		//	Msg("HTTP Request")
	}
}
