package buildinfo

import (
	"fmt"
	"net/http"
	"runtime"
)

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// AttachUserAgentHeader attaches a User-Agent header to the request
func AttachUserAgentHeader(req *http.Request) {
	agent := fmt.Sprintf("dashbrr/%s (%s %s)", Version, runtime.GOOS, runtime.GOARCH)

	req.Header.Set("User-Agent", agent)
}
