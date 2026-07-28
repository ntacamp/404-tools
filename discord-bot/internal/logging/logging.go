// Package logging provides minimal leveled logging: informational messages
// go to stdout and errors go to stderr, so process supervisors and log
// aggregators that infer severity from the output stream (systemd journal,
// Docker, cloud logging) don't misclassify routine info as errors.
package logging

import (
	"log"
	"os"
)

var (
	infoLog  = log.New(os.Stdout, "", log.LstdFlags)
	errorLog = log.New(os.Stderr, "", log.LstdFlags)
)

// Info logs a routine, non-error message to stdout.
func Info(format string, args ...any) {
	infoLog.Printf(format, args...)
}

// Error logs a failure to stderr.
func Error(format string, args ...any) {
	errorLog.Printf(format, args...)
}

// Fatal logs a failure to stderr and exits the process with status 1.
func Fatal(format string, args ...any) {
	errorLog.Printf(format, args...)
	os.Exit(1)
}
