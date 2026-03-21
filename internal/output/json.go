package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// Response is the standard JSON response envelope.
type Response struct {
	SchemaVersion int         `json:"schema_version"`
	OK            bool        `json:"ok"`
	Data          interface{} `json:"data"`
	Error         *ErrorInfo  `json:"error"`
	Warnings      []string    `json:"warnings"`
}

// ErrorInfo holds error details.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSONSuccess writes a successful JSON response to stdout.
func JSONSuccess(data interface{}, warnings []string) {
	if warnings == nil {
		warnings = []string{}
	}
	printJSON(Response{
		SchemaVersion: 1,
		OK:            true,
		Data:          data,
		Warnings:      warnings,
	})
}

// JSONError writes an error JSON response to stdout and exits.
func JSONError(code, message string, exitCode int) {
	printJSON(Response{
		SchemaVersion: 1,
		OK:            false,
		Error:         &ErrorInfo{Code: code, Message: message},
		Warnings:      []string{},
	})
	os.Exit(exitCode)
}

// JSONPartialFailure writes a response with data but ok=false.
func JSONPartialFailure(data interface{}, code, message string, warnings []string) {
	if warnings == nil {
		warnings = []string{}
	}
	printJSON(Response{
		SchemaVersion: 1,
		OK:            false,
		Data:          data,
		Error:         &ErrorInfo{Code: code, Message: message},
		Warnings:      warnings,
	})
}

func printJSON(resp Response) {
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
