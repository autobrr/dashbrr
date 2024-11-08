package logger

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Init initializes the global logger with colored output
func Init() {
	colors := map[string]string{
		"trace": "\033[36m", // Cyan
		"debug": "\033[33m", // Yellow
		"info":  "\033[34m", // Blue
		"warn":  "\033[33m", // Yellow
		"error": "\033[31m", // Red
		"fatal": "\033[35m", // Magenta
		"panic": "\033[35m", // Magenta
	}

	output := zerolog.ConsoleWriter{
		Out:     os.Stdout,
		NoColor: false,
		FormatLevel: func(i interface{}) string {
			level, ok := i.(string)
			if !ok {
				return "???"
			}
			color := colors[level]
			if color == "" {
				color = "\033[37m" // Default to white
			}
			return color + strings.ToUpper(level) + "\033[0m"
		},
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
}
