package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// GetAuthConfig returns the available authentication methods
func GetAuthConfig(c *gin.Context) {
	hasOIDC := os.Getenv("OIDC_ISSUER") != "" &&
		os.Getenv("OIDC_CLIENT_ID") != "" &&
		os.Getenv("OIDC_CLIENT_SECRET") != ""

	defaultMethod := "builtin"
	if hasOIDC {
		defaultMethod = "oidc"
	}

	c.JSON(http.StatusOK, gin.H{
		"methods": map[string]bool{
			"builtin": !hasOIDC, // Built-in auth is only available when OIDC is not configured
			"oidc":    hasOIDC,
		},
		"default": defaultMethod,
	})
}
