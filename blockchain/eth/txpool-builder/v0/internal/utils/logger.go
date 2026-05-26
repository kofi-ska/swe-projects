package utils

import (
	"log"
	"time"
)

// Info writes a formatted informational message to the standard logger.
// Each entry is timestamped using RFC3339 format.
func Info(message string) {
	log.Printf("[%s] %s\n", time.Now().Format(time.RFC3339), message)
}

// Error writes a formatted error message to the standard logger.
// Intended for concise error reporting with timestamps.
func Error(message string) {
	log.Printf("[ERROR %s] %s\n", time.Now().Format(time.RFC3339), message)
}
