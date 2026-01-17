package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Adapter implements the UIInterface for CLI mode
type Adapter struct {
	reader *bufio.Reader
}

// NewAdapter creates a new CLI adapter
func NewAdapter() *Adapter {
	return &Adapter{
		reader: bufio.NewReader(os.Stdin),
	}
}

// SendStream sends streaming content to stdout
func (a *Adapter) SendStream(content string) {
	fmt.Print(content)
}

// RequestConfirm asks user for confirmation via stdin
func (a *Adapter) RequestConfirm(action string, args string) bool {
	fmt.Printf("\n\n[Tool Call: %s]\n", action)
	fmt.Printf("Arguments: %s\n", args)
	fmt.Print("Execute? [Y/n] ")

	input, err := a.reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// RequestConfirmWithDiff asks user for confirmation with diff preview
func (a *Adapter) RequestConfirmWithDiff(path string, diffText string) bool {
	fmt.Printf("\n\n[Write File: %s]\n", path)
	fmt.Println("--- Diff Preview ---")
	fmt.Println(diffText)
	fmt.Println("--- End Diff ---")
	fmt.Print("Apply changes? [Y/n] ")

	input, err := a.reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

// ShowStatus updates the status display
func (a *Adapter) ShowStatus(status string) {
	fmt.Printf("\n[%s]\n", status)
}

// StartThinking indicates AI is thinking (no-op for CLI)
func (a *Adapter) StartThinking() {
	// CLI mode doesn't need animated status
}

// StopThinking indicates AI has finished thinking (no-op for CLI)
func (a *Adapter) StopThinking() {
	// CLI mode doesn't need animated status
}
