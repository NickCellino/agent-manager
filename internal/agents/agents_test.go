package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-manager/internal/models"
)

// testFile represents a file to create in the test directory
type testFile struct {
	path    string
	content string
	isDir   bool
	isAgent bool // true if this file should be recognized as an agent
}

// createTestDir creates a temporary directory with the specified file structure.
// files is a slice of testFile structs describing what to create.
// Returns the root directory path.
func createTestDir(t *testing.T, files []testFile) string {
	t.Helper()
	tempDir := t.TempDir()

	for _, f := range files {
		fullPath := filepath.Join(tempDir, f.path)
		if f.isDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", f.path, err)
			}
		} else {
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create parent directory for %s: %v", f.path, err)
			}
			if err := os.WriteFile(fullPath, []byte(f.content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", f.path, err)
			}
		}
	}

	return tempDir
}

func TestListAgentsInDir(t *testing.T) {
	// Declarative test directory structure
	// Only .md files whose parent directory is named "agents" should be recognized
	files := []testFile{
		// These should be recognized (parent dir is "agents")
		{path: "agents/agent1.md", content: "agent 1 content", isAgent: true},
		{path: "agents/agent2.md", content: "agent 2 content", isAgent: true},
		{path: "agents/README.md", content: "readme content", isAgent: true},
		// These should NOT be recognized (parent dir is not "agents")
		{path: "agents/subdir/agent3.md", content: "in subdir, not agents dir", isAgent: false},
		{path: "agents/subdir/nested.md", content: "deeply nested, not in agents dir", isAgent: false},
		{path: "other/some.md", content: "not in agents dir at all", isAgent: false},
		{path: "root.md", content: "not in agents dir", isAgent: false},
		// Non-.md files should be ignored regardless of parent dir
		{path: "agents/notanagent.txt", content: "should be ignored", isAgent: false},
		{path: "agents/subdir/readme.txt", content: "also ignored", isAgent: false},
		// Additional agents dirs at different nesting levels
		{path: "deep/agents/nested-agent.md", content: "nested agents dir", isAgent: true},
		{path: "deep/agents/another.md", content: "another in nested agents dir", isAgent: true},
	}

	tempDir := createTestDir(t, files)
	registry := models.Registry{
		Type:     models.RegistryTypeLocal,
		Location: tempDir,
	}

	// Test from root directory
	agents, err := listAgentsInDir(tempDir, registry)
	if err != nil {
		t.Fatalf("listAgentsInDir failed: %v", err)
	}

	// Count expected agents from the declarative structure
	var expectedAgentCount int
	expectedAgentNames := make(map[string]bool)
	for _, f := range files {
		if f.isAgent {
			expectedAgentCount++
			name := strings.TrimSuffix(filepath.Base(f.path), ".md")
			expectedAgentNames[name] = true
		}
	}

	if len(agents) != expectedAgentCount {
		t.Errorf("Expected %d agents, got %d", expectedAgentCount, len(agents))
	}

	// Check agent names
	agentNames := make(map[string]bool)
	for _, agent := range agents {
		agentNames[agent.Name] = true
	}

	// Check that all expected agents are found
	for name := range expectedAgentNames {
		if !agentNames[name] {
			t.Errorf("Expected to find agent %s", name)
		}
	}

	// Check that non-agents are NOT found
	for _, f := range files {
		if !f.isAgent && !f.isDir {
			name := strings.TrimSuffix(filepath.Base(f.path), ".md")
			if agentNames[name] {
				t.Errorf("Should NOT find %s (marked as isAgent=false)", name)
			}
		}
	}

	// Check that registry is set correctly for all found agents
	for _, agent := range agents {
		if agent.Registry.Location != tempDir {
			t.Errorf("Agent %s has wrong registry location", agent.Name)
		}
		if agent.SourcePath == "" {
			t.Errorf("Agent %s has empty SourcePath", agent.Name)
		}
		// Verify the path contains "agents" as parent directory
		parent := filepath.Base(filepath.Dir(agent.SourcePath))
		if parent != "agents" {
			t.Errorf("Agent %s parent directory is %q, expected 'agents'", agent.Name, parent)
		}
	}
}

func TestListAgentsInDir_NonExistent(t *testing.T) {
	registry := models.Registry{
		Type:     models.RegistryTypeLocal,
		Location: "/non/existent/path",
	}

	agents, err := listAgentsInDir("/non/existent/path/agents", registry)
	if err != nil {
		t.Fatalf("listAgentsInDir should not error on non-existent dir: %v", err)
	}

	if len(agents) != 0 {
		t.Errorf("Expected 0 agents for non-existent dir, got %d", len(agents))
	}
}
