package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"agent-manager/internal/models"
)

// GenerateSkillSummary generates a summary of a skill using opencode
// It reads the skill's README or other documentation files and uses opencode to summarize
func GenerateSkillSummary(skill models.Skill) (string, error) {
	// Read skill documentation
	docs, err := readSkillDocumentation(skill.SourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to read skill documentation: %w", err)
	}

	// If there's no documentation, return error
	if docs == "" {
		return "", fmt.Errorf("no documentation found for skill %s", skill.Name)
	}

	// Generate summary using opencode
	return generateSummaryWithOpencode(skill.Name, docs)
}

// readSkillDocumentation reads documentation files from a skill directory
func readSkillDocumentation(skillPath string) (string, error) {
	// Look for common documentation files
	docFiles := []string{"SKILL.md"}

	var content strings.Builder
	foundDocs := false

	for _, filename := range docFiles {
		filePath := filepath.Join(skillPath, filename)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Try next file
		}

		if foundDocs {
			content.WriteString("\n\n---\n\n")
		}
		content.WriteString(string(data))
		foundDocs = true
	}

	if !foundDocs {
		// Try to read any .md files
		entries, err := os.ReadDir(skillPath)
		if err != nil {
			return "", err
		}

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				filePath := filepath.Join(skillPath, entry.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}

				if foundDocs {
					content.WriteString("\n\n---\n\n")
				}
				content.WriteString(string(data))
				foundDocs = true
			}
		}
	}

	if !foundDocs {
		return "", fmt.Errorf("no documentation files found")
	}

	return content.String(), nil
}

// opencodeJSONEvent represents a single line from opencode's JSON output
type opencodeJSONEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Part      struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"part"`
}

// generateSummaryWithOpencode calls opencode to generate a skill summary
func generateSummaryWithOpencode(skillName string, docs string) (string, error) {
	// Create a prompt for opencode
	prompt := fmt.Sprintf(`Please provide a concise summary of this opencode skill called %q.

The skill documentation follows. Summarize what this skill does, when to use it, and any important configuration or usage notes.

---

%s

---

Please format your response as markdown with:
- A brief description (1-2 sentences)
- Key features/capabilities (bullet points)`, skillName, docs)

	// Call opencode with a timeout, using JSON format
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "opencode", "run", "--format", "json", "-m", "opencode-go/minimax-m2.5", prompt)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("opencode timed out after 30 seconds")
		}
		return "", fmt.Errorf("opencode failed: %w\nOutput: %s", err, string(output))
	}

	// Parse JSON lines to extract the text content
	summary := parseOpencodeJSONOutput(string(output))
	if summary == "" {
		return "", fmt.Errorf("opencode returned empty response")
	}

	return summary, nil
}

// parseOpencodeJSONOutput parses opencode's JSON format output and extracts the text content
func parseOpencodeJSONOutput(output string) string {
	var textParts []string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event opencodeJSONEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not valid JSON, skip this line
			continue
		}

		// Look for text events
		if event.Type == "text" && event.Part.Type == "text" && event.Part.Text != "" {
			textParts = append(textParts, event.Part.Text)
		}
	}

	return strings.TrimSpace(strings.Join(textParts, ""))
}
