package commands

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/autobrr/dashbrr/internal/database"
)

func validateHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme: must be http or https")
	}
	return parsed, nil
}

func getNextInstanceID(ctx context.Context, db *database.DB, prefix string) (string, error) {
	services, err := db.GetAllServices(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get services: %v", err)
	}

	maxNum := 0
	for _, service := range services {
		if after, ok := strings.CutPrefix(service.InstanceID, prefix); ok {
			numStr := after
			if num, err := strconv.Atoi(numStr); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("%s%d", prefix, maxNum+1), nil
}
