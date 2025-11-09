package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
)

type HistoryEntry struct {
	Display        string                 `json:"display"`
	Timestamp      int64                  `json:"timestamp"`
	Project        string                 `json:"project"`
	SessionID      string                 `json:"sessionId,omitempty"`
	PastedContents map[string]interface{} `json:"pastedContents"`
}

type Session struct {
	ID             string
	LastTimestamp  int64
	LastDisplay    string
	FirstDisplay   string
	FirstTimestamp int64
	MessageCount   int
	Entries        []*HistoryEntry
}

var (
	claudeDir   = filepath.Join(os.Getenv("HOME"), ".claude")
	historyFile = filepath.Join(claudeDir, "history.jsonl")
	projectsDir = filepath.Join(claudeDir, "projects")
)

func main() {
	fromPath := flag.String("from", "", "Project path to find sessions (default: current directory)")
	flag.Parse()

	fmt.Printf("%s╔══════════════════════════════════════════╗%s\n", colorCyan, colorReset)
	fmt.Printf("%s║   Claude Code Session Picker             ║%s\n", colorCyan, colorReset)
	fmt.Printf("%s╚══════════════════════════════════════════╝%s\n\n", colorCyan, colorReset)

	// Get current directory
	from := *fromPath
	if from == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fatal("Failed to get current directory: %v", err)
		}
		from = cwd
	}

	from = normalizePath(from)

	info("Looking for sessions in: %s", from)
	fmt.Println()

	// Load history
	entries, err := loadHistory()
	if err != nil {
		fatal("Failed to load history: %v", err)
	}

	// Find sessions
	sessions := findSessions(entries, from)
	if len(sessions) == 0 {
		warn("No sessions found for path: %s", from)
		os.Exit(0)
	}

	success("Found %d session(s)", len(sessions))
	fmt.Println()

	// Select session interactively
	session := selectSessionInteractive(sessions)
	if session == nil {
		info("Cancelled")
		os.Exit(0)
	}

	fmt.Println()

	// Show session info
	pterm.DefaultBox.WithTitle("Session Details").WithTitleTopCenter().Println(
		fmt.Sprintf(
			"ID:       %s\n"+
				"Messages: %d\n"+
				"Started:  %s\n"+
				"Last:     %s\n"+
				"Current:  %s",
			session.ID,
			session.MessageCount,
			formatTime(session.FirstTimestamp),
			formatTime(session.LastTimestamp),
			from,
		),
	)

	fmt.Println()

	// Ask for new path
	to := promptPath("Enter NEW directory path (or press Enter to just get resume command)")

	if to != "" {
		to = normalizePath(to)

		// Confirm migration
		fmt.Println()
		pterm.DefaultBox.WithTitle("Migration Plan").WithTitleTopCenter().Println(
			fmt.Sprintf("From: %s\n  To: %s", from, to),
		)

		fmt.Println()

		if !confirm("Migrate session to new directory?") {
			info("Cancelled")
			os.Exit(0)
		}

		fmt.Println()

		// Perform migration
		info("Migrating session...")
		if err := migrateSession(session, from, to); err != nil {
			fatal("Migration failed: %v", err)
		}
		success("✓ Session migrated!")

		from = to // Update for resume command
	}

	fmt.Println()

	// Show resume command
	resumeCmd := fmt.Sprintf("cd %s && claude --resume %s", from, session.ID)

	pterm.DefaultBox.WithTitle("Resume Session").WithTitleTopCenter().Println(
		fmt.Sprintf("Run this:\n\n  %s", resumeCmd),
	)

	fmt.Println()

	// Copy to clipboard
	if _, err := exec.LookPath("pbcopy"); err == nil {
		if confirm("Copy command to clipboard?") {
			cmd := exec.Command("pbcopy")
			cmd.Stdin = strings.NewReader(resumeCmd)
			if err := cmd.Run(); err == nil {
				success("✓ Copied! Paste and run.")
			}
		}
	}
}

func migrateSession(session *Session, oldPath, newPath string) error {
	// Step 1: Update history.jsonl
	if err := updateHistory(session, newPath); err != nil {
		return fmt.Errorf("failed to update history: %w", err)
	}

	// Step 2: Copy and update session files
	if err := copyAndUpdateSessionFiles(session, oldPath, newPath); err != nil {
		return fmt.Errorf("failed to copy session files: %w", err)
	}

	return nil
}

func updateHistory(session *Session, newPath string) error {
	// Read all lines
	file, err := os.Open(historyFile)
	if err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	file.Close()

	if err := scanner.Err(); err != nil {
		return err
	}

	// Update lines for this session
	updated := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			updated[i] = line
			continue
		}

		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			updated[i] = line
			continue
		}

		// Update if matches session
		if entry.SessionID == session.ID {
			entry.Project = newPath
			updatedLine, err := json.Marshal(entry)
			if err != nil {
				updated[i] = line
				continue
			}
			updated[i] = string(updatedLine)
		} else {
			updated[i] = line
		}
	}

	// Write backup
	backupFile := historyFile + ".backup"
	if err := os.WriteFile(backupFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write updated
	if err := os.WriteFile(historyFile, []byte(strings.Join(updated, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}

	return nil
}

func copyAndUpdateSessionFiles(session *Session, oldPath, newPath string) error {
	// Encode paths to directory names
	oldDir := encodeProjectPath(oldPath)
	newDir := encodeProjectPath(newPath)

	oldProjectDir := filepath.Join(projectsDir, oldDir)
	newProjectDir := filepath.Join(projectsDir, newDir)

	// Check if old directory exists
	if _, err := os.Stat(oldProjectDir); os.IsNotExist(err) {
		return fmt.Errorf("project directory not found: %s", oldProjectDir)
	}

	// Create new directory
	if err := os.MkdirAll(newProjectDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Find all session files
	pattern := filepath.Join(oldProjectDir, session.ID+"*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find session files: %w", err)
	}

	// Also copy agent files for this session
	agentFiles, _ := filepath.Glob(filepath.Join(oldProjectDir, "agent-*.jsonl"))
	matches = append(matches, agentFiles...)

	if len(matches) == 0 {
		return fmt.Errorf("no session files found")
	}

	// Copy and update each file
	for _, srcFile := range matches {
		dstFile := filepath.Join(newProjectDir, filepath.Base(srcFile))

		// Read source file
		content, err := os.ReadFile(srcFile)
		if err != nil {
			continue
		}

		// Update cwd in each line
		lines := strings.Split(string(content), "\n")
		var updatedLines []string

		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				updatedLines = append(updatedLines, line)
				continue
			}

			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				updatedLines = append(updatedLines, line)
				continue
			}

			// Update cwd if present
			if _, exists := obj["cwd"]; exists {
				obj["cwd"] = newPath
			}

			updated, err := json.Marshal(obj)
			if err != nil {
				updatedLines = append(updatedLines, line)
				continue
			}

			updatedLines = append(updatedLines, string(updated))
		}

		// Write to destination
		if err := os.WriteFile(dstFile, []byte(strings.Join(updatedLines, "\n")), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filepath.Base(dstFile), err)
		}
	}

	return nil
}

func encodeProjectPath(path string) string {
	// Remove leading slash
	encoded := strings.TrimPrefix(path, "/")
	// Replace / and . with -
	encoded = strings.ReplaceAll(encoded, "/", "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	// Add leading -
	return "-" + encoded
}

func loadHistory() ([]*HistoryEntry, error) {
	file, err := os.Open(historyFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []*HistoryEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip invalid lines
		}
		entries = append(entries, &entry)
	}

	return entries, scanner.Err()
}

func findSessions(entries []*HistoryEntry, projectPath string) []*Session {
	sessionMap := make(map[string]*Session)

	for _, entry := range entries {
		if entry.Project != projectPath {
			continue
		}

		// Use sessionId if available, otherwise treat each entry as separate
		sessionID := entry.SessionID
		if sessionID == "" {
			continue // Skip entries without session ID
		}

		session, exists := sessionMap[sessionID]
		if !exists {
			session = &Session{
				ID:      sessionID,
				Entries: []*HistoryEntry{},
			}
			sessionMap[sessionID] = session
		}

		session.Entries = append(session.Entries, entry)
		session.MessageCount++

		if entry.Timestamp > session.LastTimestamp {
			session.LastTimestamp = entry.Timestamp
			session.LastDisplay = entry.Display
		}

		if session.FirstTimestamp == 0 || entry.Timestamp < session.FirstTimestamp {
			session.FirstTimestamp = entry.Timestamp
			session.FirstDisplay = entry.Display
		}
	}

	// Convert to slice and sort by timestamp
	sessions := make([]*Session, 0, len(sessionMap))
	for _, session := range sessionMap {
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastTimestamp > sessions[j].LastTimestamp
	})

	return sessions
}

func selectSessionInteractive(sessions []*Session) *Session {
	var options []string

	for i, s := range sessions {
		sessionID := s.ID
		if len(sessionID) > 20 {
			sessionID = sessionID[:8] + "..." + sessionID[len(sessionID)-8:]
		}

		// Get first 3 messages
		firstMsgs := getFirstMessages(s, 3)
		firstContext := strings.Join(firstMsgs, " → ")
		if len(firstContext) > 150 {
			firstContext = firstContext[:147] + "..."
		}

		// Get last 3 messages
		lastMsgs := getLastMessages(s, 3)
		lastContext := strings.Join(lastMsgs, " → ")
		if len(lastContext) > 150 {
			lastContext = lastContext[:147] + "..."
		}

		option := fmt.Sprintf(
			"[%d] %s | %d msgs | %s → %s\n    Start: %s\n    Last:  %s",
			i+1,
			sessionID,
			s.MessageCount,
			formatTime(s.FirstTimestamp),
			formatTime(s.LastTimestamp),
			firstContext,
			lastContext,
		)

		options = append(options, option)
	}

	pterm.DefaultSection.Println("Select session (↑/↓ arrows, Enter to confirm)")

	result, err := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText("Choose a session to migrate").
		Show()

	if err != nil {
		return nil
	}

	// Parse selection
	for i, opt := range options {
		if opt == result {
			return sessions[i]
		}
	}

	return nil
}

func getFirstMessages(session *Session, count int) []string {
	// Sort entries by timestamp
	sorted := make([]*HistoryEntry, len(session.Entries))
	copy(sorted, session.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	var messages []string
	for i := 0; i < count && i < len(sorted); i++ {
		msg := sorted[i].Display
		msg = strings.TrimSpace(msg)
		if msg != "" {
			// Truncate long messages
			if len(msg) > 80 {
				msg = msg[:77] + "..."
			}
			messages = append(messages, msg)
		}
	}

	return messages
}

func getLastMessages(session *Session, count int) []string {
	// Sort entries by timestamp (descending)
	sorted := make([]*HistoryEntry, len(session.Entries))
	copy(sorted, session.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp > sorted[j].Timestamp
	})

	var messages []string
	for i := 0; i < count && i < len(sorted); i++ {
		msg := sorted[i].Display
		msg = strings.TrimSpace(msg)
		if msg != "" {
			// Truncate long messages
			if len(msg) > 80 {
				msg = msg[:77] + "..."
			}
			messages = append(messages, msg)
		}
	}

	// Reverse to show chronologically
	for i := len(messages)/2 - 1; i >= 0; i-- {
		opp := len(messages) - 1 - i
		messages[i], messages[opp] = messages[opp], messages[i]
	}

	return messages
}

func normalizePath(path string) string {
	// Expand ~
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return absPath
}

func promptPath(prompt string) string {
	fmt.Printf("%s: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func confirm(prompt string) bool {
	fmt.Printf("%s%s (Y/n): %s", colorYellow, prompt, colorReset)
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	// Default to yes if empty
	if response == "" {
		return true
	}
	return response == "y" || response == "yes"
}

func formatTime(timestamp int64) string {
	t := time.Unix(timestamp/1000, 0)
	return t.Format("2006-01-02 15:04")
}

func info(format string, args ...interface{}) {
	fmt.Printf("%sℹ %s%s\n", colorBlue, fmt.Sprintf(format, args...), colorReset)
}

func success(format string, args ...interface{}) {
	fmt.Printf("%s✓ %s%s\n", colorGreen, fmt.Sprintf(format, args...), colorReset)
}

func warn(format string, args ...interface{}) {
	fmt.Printf("%s⚠ %s%s\n", colorYellow, fmt.Sprintf(format, args...), colorReset)
}

func fatal(format string, args ...interface{}) {
	fmt.Printf("%s✗ %s%s\n", colorRed, fmt.Sprintf(format, args...), colorReset)
	os.Exit(1)
}
