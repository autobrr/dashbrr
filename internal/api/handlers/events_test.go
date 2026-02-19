package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/autobrr/dashbrr/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEncodeHealthAsSSE(t *testing.T) {
	h := models.ServiceHealth{
		ServiceID:   "plex-1",
		Status:      "online",
		Message:     "Healthy",
		LastChecked: time.Now(),
		Stats: map[string]interface{}{
			"plex": map[string]interface{}{"sessions": []interface{}{}},
		},
	}

	payload := EncodeHealthAsSSE(h)
	s := string(payload)
	require.True(t, strings.HasPrefix(s, "data: "))
	require.True(t, strings.HasSuffix(s, "\n\n"))

	// Decode JSON after "data: "
	raw := strings.TrimSuffix(strings.TrimPrefix(s, "data: "), "\n\n")
	var decoded models.ServiceHealth
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))
	require.Equal(t, h.ServiceID, decoded.ServiceID)
	require.Equal(t, h.Status, decoded.Status)
	require.Equal(t, h.Message, decoded.Message)
}

func TestIsExpectedSSEWriteError_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.True(t, isExpectedSSEWriteError(ctx, fmt.Errorf("write failed")))
}

func TestIsExpectedSSEWriteError_ConnectionClosed(t *testing.T) {
	ctx := context.Background()

	require.True(t, isExpectedSSEWriteError(ctx, net.ErrClosed))
	require.True(t, isExpectedSSEWriteError(ctx, syscall.EPIPE))
	require.True(t, isExpectedSSEWriteError(ctx, syscall.ECONNRESET))
}

func TestIsExpectedSSEWriteError_Unexpected(t *testing.T) {
	ctx := context.Background()

	require.False(t, isExpectedSSEWriteError(ctx, fmt.Errorf("unexpected")))
}
