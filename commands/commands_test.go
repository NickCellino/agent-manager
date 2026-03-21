package commands_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryPath holds the path to the compiled agent-manager binary used in all tests.
var binaryPath string

// gitNoPrompt disables interactive git credential prompts in all test child
// processes, so that clone attempts against private/unreachable repos fail
// fast (exit 128) instead of blocking the test run indefinitely.
const gitNoPrompt = "GIT_TERMINAL_PROMPT=0"

// TestMain builds the binary once before running all tests and removes it afterward.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "agent-manager-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create temp dir:", err)
		os.Exit(1)
	}

	binaryPath = filepath.Join(tmpDir, "agent-manager")

	// The test package lives in commands/, so the repo root is one level up.
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to determine repo root:", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "failed to build agent-manager:", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// testEnv holds isolated temporary directories for a single test.
type testEnv struct {
	xdgDataHome string // isolated global config (registries.json lives here)
	projectDir  string // isolated project dir (lock file and skills installed here)
}

// newTestEnv creates a fresh isolated environment for a test.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return &testEnv{
		xdgDataHome: t.TempDir(),
		projectDir:  t.TempDir(),
	}
}

// run executes the binary with the provided arguments inside the test environment.
// It returns stdout, stderr, and the process exit code.
func (e *testEnv) run(args ...string) (stdout, stderr string, code int) {
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = e.projectDir
	cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+e.xdgDataHome, gitNoPrompt)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return outBuf.String(), errBuf.String(), code
}

// makeLocalRegistry creates a temporary directory structured as a skill registry
// and populates it with subdirectories for each named skill.
func makeLocalRegistry(t *testing.T, skillNames ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range skillNames {
		skillDir := filepath.Join(dir, ".opencode", "skills", name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("makeLocalRegistry: failed to create skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte("name: "+name+"\n"), 0644); err != nil {
			t.Fatalf("makeLocalRegistry: failed to write skill file: %v", err)
		}
	}
	return dir
}

// ---- Registry tests ----

func TestRegistryList_Empty(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("registry", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No registries configured") {
		t.Errorf("expected 'No registries configured' in output, got:\n%s", out)
	}
}

func TestRegistryAdd_Local(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")

	out, _, code := env.run("registry", "add", "local", regDir)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Added local registry") {
		t.Errorf("expected 'Added local registry' in output, got:\n%s", out)
	}
	if !strings.Contains(out, regDir) {
		t.Errorf("expected registry path %q in output, got:\n%s", regDir, out)
	}
}

func TestRegistryAdd_InvalidType(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("registry", "add", "badtype", "/some/path")
	if code == 0 {
		t.Fatal("expected non-zero exit for invalid type")
	}
	if !strings.Contains(errOut, "invalid registry type") {
		t.Errorf("expected 'invalid registry type' in stderr, got:\n%s", errOut)
	}
}

func TestRegistryAdd_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("registry", "add", "github")
	if code == 0 {
		t.Fatal("expected non-zero exit when location arg is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestRegistryAdd_Duplicate(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")

	env.run("registry", "add", "local", regDir) // first add succeeds
	_, errOut, code := env.run("registry", "add", "local", regDir)
	if code == 0 {
		t.Fatal("expected non-zero exit when adding duplicate registry")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got:\n%s", errOut)
	}
}

func TestRegistryList_AfterAdd(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")

	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("registry", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "[local]") {
		t.Errorf("expected '[local]' in output, got:\n%s", out)
	}
	if !strings.Contains(out, regDir) {
		t.Errorf("expected registry path %q in output, got:\n%s", regDir, out)
	}
	if !strings.Contains(out, "1 total") {
		t.Errorf("expected '1 total' in output, got:\n%s", out)
	}
}

func TestRegistryRemove(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")

	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("registry", "remove", "local", regDir)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Removed local registry") {
		t.Errorf("expected 'Removed local registry' in output, got:\n%s", out)
	}

	// Verify it's gone
	listOut, _, _ := env.run("registry", "list")
	if !strings.Contains(listOut, "No registries configured") {
		t.Errorf("expected registry to be absent after removal, list output:\n%s", listOut)
	}
}

func TestRegistryRemove_NotFound(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("registry", "remove", "local", "/nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when removing nonexistent registry")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

// ---- Skills tests ----

func TestSkillsList_NoRegistries(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("skills", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No skills found") {
		t.Errorf("expected 'No skills found' in output, got:\n%s", out)
	}
}

func TestSkillsList(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "skill-a", "skill-b")
	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("skills", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "skill-a") {
		t.Errorf("expected 'skill-a' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "skill-b") {
		t.Errorf("expected 'skill-b' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in output, got:\n%s", out)
	}
}

func TestSkillsInstalled_Empty(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("skills", "installed")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No skills managed") {
		t.Errorf("expected 'No skills managed' in output, got:\n%s", out)
	}
}

func TestSkillsAdd(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("skills", "add", "my-skill")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed skill "my-skill"`) {
		t.Errorf("expected 'Installed skill' in output, got:\n%s", out)
	}

	// Verify the skill directory was created on the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "my-skill")
	if _, err := os.Lstat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected skill to exist at %s after install", installedPath)
	}
}

func TestSkillsInstalled_AfterAdd(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)
	env.run("skills", "add", "my-skill")

	out, _, code := env.run("skills", "installed")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected 'my-skill' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 total") {
		t.Errorf("expected '1 total' in output, got:\n%s", out)
	}
}

func TestSkillsAdd_AlreadyInstalled(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)
	env.run("skills", "add", "my-skill") // first install

	out, _, code := env.run("skills", "add", "my-skill") // second install
	if code != 0 {
		t.Fatalf("expected exit 0 (idempotent), got %d", code)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' in output, got:\n%s", out)
	}
}

func TestSkillsAdd_NotFound(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "other-skill")
	env.run("registry", "add", "local", regDir)

	_, errOut, code := env.run("skills", "add", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when skill not found")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

func TestSkillsAdd_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("skills", "add")
	if code == 0 {
		t.Fatal("expected non-zero exit when skill name is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestSkillsRemove(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)
	env.run("skills", "add", "my-skill")

	out, _, code := env.run("skills", "remove", "my-skill")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `Removed skill "my-skill"`) {
		t.Errorf("expected 'Removed skill' in output, got:\n%s", out)
	}

	// Verify the skill was removed from the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "my-skill")
	if _, err := os.Lstat(installedPath); !os.IsNotExist(err) {
		t.Errorf("expected skill to be absent at %s after removal", installedPath)
	}

	// Verify it's no longer in the installed list
	listOut, _, _ := env.run("skills", "installed")
	if !strings.Contains(listOut, "No skills managed") {
		t.Errorf("expected 'No skills managed' after removal, list output:\n%s", listOut)
	}
}

func TestSkillsRemove_NotManaged(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("skills", "remove", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when removing unmanaged skill")
	}
	if !strings.Contains(errOut, "not managed") {
		t.Errorf("expected 'not managed' in stderr, got:\n%s", errOut)
	}
}

func TestSkillsRemove_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("skills", "remove")
	if code == 0 {
		t.Fatal("expected non-zero exit when skill name is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestSkillsAdd_MultipleRegistries_RequiresFlag(t *testing.T) {
	env := newTestEnv(t)
	reg1 := makeLocalRegistry(t, "shared-skill")
	reg2 := makeLocalRegistry(t, "shared-skill")
	env.run("registry", "add", "local", reg1)
	env.run("registry", "add", "local", reg2)

	_, errOut, code := env.run("skills", "add", "shared-skill")
	if code == 0 {
		t.Fatal("expected non-zero exit when skill exists in multiple registries without --registry flag")
	}
	if !strings.Contains(errOut, "--registry") {
		t.Errorf("expected '--registry' hint in stderr, got:\n%s", errOut)
	}
}

func TestSkillsAdd_MultipleRegistries_WithFlag(t *testing.T) {
	env := newTestEnv(t)
	reg1 := makeLocalRegistry(t, "shared-skill")
	reg2 := makeLocalRegistry(t, "shared-skill")
	env.run("registry", "add", "local", reg1)
	env.run("registry", "add", "local", reg2)

	// Flag must precede the positional argument
	out, _, code := env.run("skills", "add", "--registry", "local:"+reg1, "shared-skill")
	if code != 0 {
		t.Fatalf("expected exit 0 with --registry flag, got %d", code)
	}
	if !strings.Contains(out, `Installed skill "shared-skill"`) {
		t.Errorf("expected 'Installed skill' in output, got:\n%s", out)
	}
}

// ---- GitHub Registry tests ----

// addGitHubRegistry adds the NickCellino/opencode-e2e-test-registry GitHub registry to env,
// then runs `skills list` to trigger a clone of the repo.  If the expected skills
// do not appear in the output (e.g. the repo is private or network is unavailable),
// the test fails. On success it returns the `skills list` stdout so
// the caller can make additional assertions without running the command a second time.
func addGitHubRegistry(t *testing.T, env *testEnv) string {
	t.Helper()
	env.run("registry", "add", "github", "NickCellino/opencode-e2e-test-registry")
	out, errOut, _ := env.run("skills", "list")
	if !strings.Contains(out, "e2e-test-skill") {
		t.Fatalf("expected 'e2e-test-skill' in skills list output; NickCellino/opencode-e2e-test-registry may not be accessible.\nstdout: %s\nstderr: %s", out, errOut)
	}
	if !strings.Contains(out, "sample-validation-skill") {
		t.Fatalf("expected 'sample-validation-skill' in skills list output; NickCellino/opencode-e2e-test-registry may not be accessible.\nstdout: %s\nstderr: %s", out, errOut)
	}
	return out
}

func TestRegistryAdd_GitHub(t *testing.T) {
	env := newTestEnv(t)

	out, _, code := env.run("registry", "add", "github", "NickCellino/opencode-e2e-test-registry")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Added github registry") {
		t.Errorf("expected 'Added github registry' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "NickCellino/opencode-e2e-test-registry") {
		t.Errorf("expected 'NickCellino/opencode-e2e-test-registry' in output, got:\n%s", out)
	}

	// Verify it appears in the list (no network needed — registry config is local)
	listOut, _, _ := env.run("registry", "list")
	if !strings.Contains(listOut, "[github]") {
		t.Errorf("expected '[github]' in registry list output, got:\n%s", listOut)
	}
	if !strings.Contains(listOut, "NickCellino/opencode-e2e-test-registry") {
		t.Errorf("expected 'NickCellino/opencode-e2e-test-registry' in registry list output, got:\n%s", listOut)
	}
}

func TestSkillsList_GitHub(t *testing.T) {
	env := newTestEnv(t)
	out := addGitHubRegistry(t, env)

	// Verify both expected skills are present with exact names
	if !strings.Contains(out, "e2e-test-skill") {
		t.Errorf("expected 'e2e-test-skill' in skills list output, got:\n%s", out)
	}
	if !strings.Contains(out, "sample-validation-skill") {
		t.Errorf("expected 'sample-validation-skill' in skills list output, got:\n%s", out)
	}
	// Verify registry info is shown for skills
	if !strings.Contains(out, "[github: NickCellino/opencode-e2e-test-registry]") {
		t.Errorf("expected '[github: NickCellino/opencode-e2e-test-registry]' in skills list output, got:\n%s", out)
	}
	// Verify total count
	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in skills list output, got:\n%s", out)
	}
}

func TestSkillsAdd_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistry(t, env)

	out, _, code := env.run("skills", "add", "e2e-test-skill")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed skill "e2e-test-skill"`) {
		t.Errorf("expected 'Installed skill \"e2e-test-skill\"' in output, got:\n%s", out)
	}

	// GitHub registries copy the skill directory (not a symlink)
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "e2e-test-skill")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("expected e2e-test-skill skill to exist at %s: %v", installedPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected a regular directory (not symlink) for a GitHub registry skill at %s", installedPath)
	}
}

func TestSkillsRemove_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistry(t, env)

	env.run("skills", "add", "e2e-test-skill")

	out, _, code := env.run("skills", "remove", "e2e-test-skill")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Removed skill "e2e-test-skill"`) {
		t.Errorf("expected 'Removed skill \"e2e-test-skill\"' in output, got:\n%s", out)
	}

	// Verify the skill directory was removed from the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "e2e-test-skill")
	if _, err := os.Lstat(installedPath); !os.IsNotExist(err) {
		t.Errorf("expected e2e-test-skill skill to be absent at %s after removal", installedPath)
	}

	// Verify it's no longer in the installed list
	installedOut, _, _ := env.run("skills", "installed")
	if strings.Contains(installedOut, "e2e-test-skill") {
		t.Errorf("expected 'e2e-test-skill' to be absent from installed list after removal, got:\n%s", installedOut)
	}
}

func TestFullGitHubRegistryWorkflow(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistry(t, env)

	// Install the e2e-test-skill
	env.run("skills", "add", "e2e-test-skill")

	// Verify it appears in the installed list
	installedOut, _, code := env.run("skills", "installed")
	if code != 0 {
		t.Fatalf("skills installed: expected exit 0, got %d", code)
	}
	if !strings.Contains(installedOut, "e2e-test-skill") {
		t.Errorf("expected 'e2e-test-skill' in installed list, got:\n%s", installedOut)
	}
	if !strings.Contains(installedOut, "[github: NickCellino/opencode-e2e-test-registry]") {
		t.Errorf("expected registry info in installed list, got:\n%s", installedOut)
	}

	// Installing again should be idempotent
	idempotentOut, _, idempotentCode := env.run("skills", "add", "e2e-test-skill")
	if idempotentCode != 0 {
		t.Fatalf("expected exit 0 on duplicate install, got %d", idempotentCode)
	}
	if !strings.Contains(idempotentOut, "already installed") {
		t.Errorf("expected 'already installed' on duplicate install, got:\n%s", idempotentOut)
	}

	// Remove the skill
	env.run("skills", "remove", "e2e-test-skill")

	// Nothing should be installed
	afterRemove, _, _ := env.run("skills", "installed")
	if !strings.Contains(afterRemove, "No skills managed") {
		t.Errorf("expected 'No skills managed' after removal, got:\n%s", afterRemove)
	}
}

// ---- Full local workflow ----

func TestFullRegistryAndSkillsWorkflow(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "alpha", "beta")

	// Add registry
	env.run("registry", "add", "local", regDir)

	// Both skills are listed
	listOut, _, _ := env.run("skills", "list")
	if !strings.Contains(listOut, "alpha") || !strings.Contains(listOut, "beta") {
		t.Errorf("expected both skills in list, got:\n%s", listOut)
	}

	// Install alpha
	env.run("skills", "add", "alpha")

	// Only alpha is installed
	installedOut, _, _ := env.run("skills", "installed")
	if !strings.Contains(installedOut, "alpha") {
		t.Errorf("expected alpha in installed list, got:\n%s", installedOut)
	}
	if strings.Contains(installedOut, "beta") {
		t.Errorf("did not expect beta in installed list, got:\n%s", installedOut)
	}

	// Remove alpha
	env.run("skills", "remove", "alpha")

	// Nothing installed
	afterRemove, _, _ := env.run("skills", "installed")
	if !strings.Contains(afterRemove, "No skills managed") {
		t.Errorf("expected 'No skills managed' after removal, got:\n%s", afterRemove)
	}
}

// ---- Agents helpers ----

// makeLocalRegistryWithAgents creates a temporary directory structured as an agent registry
// and populates it with .md files for each named agent.
func makeLocalRegistryWithAgents(t *testing.T, agentNames ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range agentNames {
		agentsDir := filepath.Join(dir, ".opencode", "agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("makeLocalRegistryWithAgents: failed to create agents dir: %v", err)
		}
		agentFile := filepath.Join(agentsDir, name+".md")
		content := "---\ndescription: " + name + "\nmode: subagent\n---\n\nYou are " + name + ".\n"
		if err := os.WriteFile(agentFile, []byte(content), 0644); err != nil {
			t.Fatalf("makeLocalRegistryWithAgents: failed to write agent file: %v", err)
		}
	}
	return dir
}

// addGitHubRegistryForAgents adds the NickCellino/opencode-e2e-test-registry GitHub registry to env,
// then runs `agents list` to trigger a clone of the repo. If the expected agents are not found,
// the test fails. On success it returns the `agents list` stdout.
func addGitHubRegistryForAgents(t *testing.T, env *testEnv) string {
	t.Helper()
	env.run("registry", "add", "github", "NickCellino/opencode-e2e-test-registry")
	out, errOut, _ := env.run("agents", "list")
	if errOut != "" {
		t.Fatalf("expected empty error output, got %v", errOut)
	}
	if !strings.Contains(out, "e2e-test-agent") {
		t.Fatalf("expected 'e2e-test-agent' in agents list output; NickCellino/opencode-e2e-test-registry may not be accessible.\nstdout: %s\nstderr: %s", out, errOut)
	}
	if !strings.Contains(out, "sample-helper-agent") {
		t.Fatalf("expected 'sample-helper-agent' in agents list output; NickCellino/opencode-e2e-test-registry may not be accessible.\nstdout: %s\nstderr: %s", out, errOut)
	}
	return out
}

// ---- Agents tests ----

func TestAgentsList_NoRegistries(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("agents", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No agents found") {
		t.Errorf("expected 'No agents found' in output, got:\n%s", out)
	}
}

func TestAgentsList(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "agent-a", "agent-b")
	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("agents", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "agent-a") {
		t.Errorf("expected 'agent-a' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "agent-b") {
		t.Errorf("expected 'agent-b' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in output, got:\n%s", out)
	}
}

func TestAgentsInstalled_Empty(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("agents", "installed")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No agents managed") {
		t.Errorf("expected 'No agents managed' in output, got:\n%s", out)
	}
}

func TestAgentsAdd(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)

	out, _, code := env.run("agents", "add", "my-agent")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed agent "my-agent"`) {
		t.Errorf("expected 'Installed agent' in output, got:\n%s", out)
	}

	// Verify the agent .md file was created on the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", "my-agent.md")
	if _, err := os.Lstat(installedPath); os.IsNotExist(err) {
		t.Errorf("expected agent to exist at %s after install", installedPath)
	}
}

func TestAgentsInstalled_AfterAdd(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)
	env.run("agents", "add", "my-agent")

	out, _, code := env.run("agents", "installed")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "my-agent") {
		t.Errorf("expected 'my-agent' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 total") {
		t.Errorf("expected '1 total' in output, got:\n%s", out)
	}
}

func TestAgentsAdd_AlreadyInstalled(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)
	env.run("agents", "add", "my-agent") // first install

	out, _, code := env.run("agents", "add", "my-agent") // second install
	if code != 0 {
		t.Fatalf("expected exit 0 (idempotent), got %d", code)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' in output, got:\n%s", out)
	}
}

func TestAgentsAdd_NotFound(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "other-agent")
	env.run("registry", "add", "local", regDir)

	_, errOut, code := env.run("agents", "add", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when agent not found")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

func TestAgentsAdd_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("agents", "add")
	if code == 0 {
		t.Fatal("expected non-zero exit when agent name is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestAgentsRemove(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)
	env.run("agents", "add", "my-agent")

	out, _, code := env.run("agents", "remove", "my-agent")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `Removed agent "my-agent"`) {
		t.Errorf("expected 'Removed agent' in output, got:\n%s", out)
	}

	// Verify the agent .md file was removed from the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", "my-agent.md")
	if _, err := os.Lstat(installedPath); !os.IsNotExist(err) {
		t.Errorf("expected agent to be absent at %s after removal", installedPath)
	}

	// Verify it's no longer in the installed list
	listOut, _, _ := env.run("agents", "installed")
	if !strings.Contains(listOut, "No agents managed") {
		t.Errorf("expected 'No agents managed' after removal, list output:\n%s", listOut)
	}
}

func TestAgentsRemove_NotManaged(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("agents", "remove", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when removing unmanaged agent")
	}
	if !strings.Contains(errOut, "not managed") {
		t.Errorf("expected 'not managed' in stderr, got:\n%s", errOut)
	}
}

func TestAgentsRemove_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("agents", "remove")
	if code == 0 {
		t.Fatal("expected non-zero exit when agent name is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestAgentsAdd_MultipleRegistries_RequiresFlag(t *testing.T) {
	env := newTestEnv(t)
	reg1 := makeLocalRegistryWithAgents(t, "shared-agent")
	reg2 := makeLocalRegistryWithAgents(t, "shared-agent")
	env.run("registry", "add", "local", reg1)
	env.run("registry", "add", "local", reg2)

	_, errOut, code := env.run("agents", "add", "shared-agent")
	if code == 0 {
		t.Fatal("expected non-zero exit when agent exists in multiple registries without --registry flag")
	}
	if !strings.Contains(errOut, "--registry") {
		t.Errorf("expected '--registry' hint in stderr, got:\n%s", errOut)
	}
}

func TestAgentsAdd_MultipleRegistries_WithFlag(t *testing.T) {
	env := newTestEnv(t)
	reg1 := makeLocalRegistryWithAgents(t, "shared-agent")
	reg2 := makeLocalRegistryWithAgents(t, "shared-agent")
	env.run("registry", "add", "local", reg1)
	env.run("registry", "add", "local", reg2)

	out, _, code := env.run("agents", "add", "--registry", "local:"+reg1, "shared-agent")
	if code != 0 {
		t.Fatalf("expected exit 0 with --registry flag, got %d", code)
	}
	if !strings.Contains(out, `Installed agent "shared-agent"`) {
		t.Errorf("expected 'Installed agent' in output, got:\n%s", out)
	}
}

func TestAgentsAdd_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistryForAgents(t, env)

	out, _, code := env.run("agents", "add", "e2e-test-agent")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed agent "e2e-test-agent"`) {
		t.Errorf("expected 'Installed agent \"e2e-test-agent\"' in output, got:\n%s", out)
	}

	// GitHub registries copy the agent file (not a symlink)
	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", "e2e-test-agent.md")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("expected agent file to exist at %s: %v", installedPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected a regular file (not symlink) for a GitHub registry agent at %s", installedPath)
	}
}

func TestAgentsRemove_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistryForAgents(t, env)

	env.run("agents", "add", "e2e-test-agent")

	out, _, code := env.run("agents", "remove", "e2e-test-agent")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Removed agent "e2e-test-agent"`) {
		t.Errorf("expected 'Removed agent \"e2e-test-agent\"' in output, got:\n%s", out)
	}

	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", "e2e-test-agent.md")
	if _, err := os.Lstat(installedPath); !os.IsNotExist(err) {
		t.Errorf("expected agent file to be absent at %s after removal", installedPath)
	}
}

func TestFullRegistryAndAgentsWorkflow(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "alpha", "beta")

	// Add registry
	env.run("registry", "add", "local", regDir)

	// Both agents are listed
	listOut, _, _ := env.run("agents", "list")
	if !strings.Contains(listOut, "alpha") || !strings.Contains(listOut, "beta") {
		t.Errorf("expected both agents in list, got:\n%s", listOut)
	}

	// Install alpha
	env.run("agents", "add", "alpha")

	// Only alpha is installed
	installedOut, _, _ := env.run("agents", "installed")
	if !strings.Contains(installedOut, "alpha") {
		t.Errorf("expected alpha in installed list, got:\n%s", installedOut)
	}
	if strings.Contains(installedOut, "beta") {
		t.Errorf("did not expect beta in installed list, got:\n%s", installedOut)
	}

	// Remove alpha
	env.run("agents", "remove", "alpha")

	// Nothing installed
	afterRemove, _, _ := env.run("agents", "installed")
	if !strings.Contains(afterRemove, "No agents managed") {
		t.Errorf("expected 'No agents managed' after removal, got:\n%s", afterRemove)
	}
}

// ---- Update helpers ----

// mustExec runs a command and fatals the test on failure.
func mustExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %v failed: %v\nOutput: %s", args, err, out)
	}
}

// makeGitUpstreamWithSkill initialises a git repo with an initial skill commit,
// clones it to the expected github-registries path inside env, and returns the
// upstream repo path (so tests can push further commits).
func makeGitUpstreamWithSkill(t *testing.T, env *testEnv, owner, repo, skillName, content string) string {
	t.Helper()

	upstreamDir := t.TempDir()
	mustExec(t, upstreamDir, "git", "init")
	mustExec(t, upstreamDir, "git", "config", "user.email", "test@test.com")
	mustExec(t, upstreamDir, "git", "config", "user.name", "Test")

	skillDir := filepath.Join(upstreamDir, ".opencode", "skills", skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("makeGitUpstreamWithSkill: failed to create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("makeGitUpstreamWithSkill: failed to write skill file: %v", err)
	}

	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "initial")

	// Clone to the github-registries path that agent-manager expects
	cloneDir := filepath.Join(env.xdgDataHome, "agent-manager", "github-registries", owner, repo)
	if err := os.MkdirAll(filepath.Dir(cloneDir), 0755); err != nil {
		t.Fatalf("makeGitUpstreamWithSkill: failed to create parent dir: %v", err)
	}
	mustExec(t, "", "git", "clone", upstreamDir, cloneDir)

	return upstreamDir
}

// makeGitUpstreamWithAgent initialises a git repo with an initial agent commit,
// clones it to the expected github-registries path inside env, and returns the
// upstream repo path (so tests can push further commits).
func makeGitUpstreamWithAgent(t *testing.T, env *testEnv, owner, repo, agentName, content string) string {
	t.Helper()

	upstreamDir := t.TempDir()
	mustExec(t, upstreamDir, "git", "init")
	mustExec(t, upstreamDir, "git", "config", "user.email", "test@test.com")
	mustExec(t, upstreamDir, "git", "config", "user.name", "Test")

	agentsDir := filepath.Join(upstreamDir, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("makeGitUpstreamWithAgent: failed to create agents dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, agentName+".md"), []byte(content), 0644); err != nil {
		t.Fatalf("makeGitUpstreamWithAgent: failed to write agent file: %v", err)
	}

	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "initial")

	cloneDir := filepath.Join(env.xdgDataHome, "agent-manager", "github-registries", owner, repo)
	if err := os.MkdirAll(filepath.Dir(cloneDir), 0755); err != nil {
		t.Fatalf("makeGitUpstreamWithAgent: failed to create parent dir: %v", err)
	}
	mustExec(t, "", "git", "clone", upstreamDir, cloneDir)

	return upstreamDir
}

// ---- Skills update tests ----

func TestSkillsUpdate_NoManagedSkills(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("skills", "update")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No skills managed") {
		t.Errorf("expected 'No skills managed' in output, got:\n%s", out)
	}
}

func TestSkillsUpdate_NoGitHubSkills(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)
	env.run("skills", "add", "my-skill")

	out, _, code := env.run("skills", "update")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No skills from GitHub registries") {
		t.Errorf("expected 'No skills from GitHub registries' in output, got:\n%s", out)
	}
}

func TestSkillsUpdate_NotManaged(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("skills", "update", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when skill not managed")
	}
	if !strings.Contains(errOut, "not managed") {
		t.Errorf("expected 'not managed' in stderr, got:\n%s", errOut)
	}
}

func TestSkillsUpdate_SkipsLocalRegistry(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)
	env.run("skills", "add", "my-skill")

	out, _, code := env.run("skills", "update", "my-skill")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Skipping") {
		t.Errorf("expected 'Skipping' in output for non-GitHub skill, got:\n%s", out)
	}
}

func TestSkillsUpdate_GitHub_SpecificSkill(t *testing.T) {
	env := newTestEnv(t)
	owner, repo, skillName := "test-owner", "test-repo", "my-skill"

	upstreamDir := makeGitUpstreamWithSkill(t, env, owner, repo, skillName, "version: 1\n")

	// Register as a github registry and install the skill
	env.run("registry", "add", "github", owner+"/"+repo)
	out, _, code := env.run("skills", "add", skillName)
	if code != 0 {
		t.Fatalf("skills add failed: %s", out)
	}

	// Verify initial content
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", skillName, "skill.yaml")
	content, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed skill file: %v", err)
	}
	if string(content) != "version: 1\n" {
		t.Fatalf("expected 'version: 1\\n' before update, got %q", string(content))
	}

	// Push an updated version to the upstream
	if err := os.WriteFile(filepath.Join(upstreamDir, ".opencode", "skills", skillName, "skill.yaml"), []byte("version: 2\n"), 0644); err != nil {
		t.Fatalf("failed to write updated skill file: %v", err)
	}
	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "update skill")

	// Run skills update for the specific skill
	out, _, code = env.run("skills", "update", skillName)
	if code != 0 {
		t.Fatalf("expected exit 0 from skills update, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, fmt.Sprintf("Updated skill %q", skillName)) {
		t.Errorf("expected 'Updated skill' in output, got:\n%s", out)
	}

	// Verify updated content
	content, err = os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed skill file after update: %v", err)
	}
	if string(content) != "version: 2\n" {
		t.Fatalf("expected 'version: 2\\n' after update, got %q", string(content))
	}
}

func TestSkillsUpdate_GitHub_AllSkills(t *testing.T) {
	env := newTestEnv(t)
	owner, repo, skillName := "test-owner2", "test-repo2", "all-update-skill"

	upstreamDir := makeGitUpstreamWithSkill(t, env, owner, repo, skillName, "v: 1\n")
	env.run("registry", "add", "github", owner+"/"+repo)
	env.run("skills", "add", skillName)

	// Push an update
	if err := os.WriteFile(filepath.Join(upstreamDir, ".opencode", "skills", skillName, "skill.yaml"), []byte("v: 2\n"), 0644); err != nil {
		t.Fatalf("failed to write updated skill: %v", err)
	}
	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "update")

	// Run skills update with no args (updates all GitHub skills)
	out, _, code := env.run("skills", "update")
	if code != 0 {
		t.Fatalf("expected exit 0 from skills update, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, fmt.Sprintf("Updated skill %q", skillName)) {
		t.Errorf("expected 'Updated skill' in output, got:\n%s", out)
	}

	// Verify updated content
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", skillName, "skill.yaml")
	content, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed skill file after update: %v", err)
	}
	if string(content) != "v: 2\n" {
		t.Fatalf("expected 'v: 2\\n' after update, got %q", string(content))
	}
}

// ---- Agents update tests ----

func TestAgentsUpdate_NoManagedAgents(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("agents", "update")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No agents managed") {
		t.Errorf("expected 'No agents managed' in output, got:\n%s", out)
	}
}

func TestAgentsUpdate_NoGitHubAgents(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)
	env.run("agents", "add", "my-agent")

	out, _, code := env.run("agents", "update")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No agents from GitHub registries") {
		t.Errorf("expected 'No agents from GitHub registries' in output, got:\n%s", out)
	}
}

func TestAgentsUpdate_NotManaged(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("agents", "update", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when agent not managed")
	}
	if !strings.Contains(errOut, "not managed") {
		t.Errorf("expected 'not managed' in stderr, got:\n%s", errOut)
	}
}

func TestAgentsUpdate_SkipsLocalRegistry(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "my-agent")
	env.run("registry", "add", "local", regDir)
	env.run("agents", "add", "my-agent")

	out, _, code := env.run("agents", "update", "my-agent")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Skipping") {
		t.Errorf("expected 'Skipping' in output for non-GitHub agent, got:\n%s", out)
	}
}

func TestAgentsUpdate_GitHub_SpecificAgent(t *testing.T) {
	env := newTestEnv(t)
	owner, repo, agentName := "test-owner-a", "test-repo-a", "my-agent"

	upstreamDir := makeGitUpstreamWithAgent(t, env, owner, repo, agentName, "version 1 content\n")

	env.run("registry", "add", "github", owner+"/"+repo)
	out, _, code := env.run("agents", "add", agentName)
	if code != 0 {
		t.Fatalf("agents add failed: %s", out)
	}

	// Verify initial content
	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", agentName+".md")
	content, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed agent file: %v", err)
	}
	if string(content) != "version 1 content\n" {
		t.Fatalf("expected 'version 1 content\\n' before update, got %q", string(content))
	}

	// Push an updated version
	agentsDir := filepath.Join(upstreamDir, ".opencode", "agents")
	if err := os.WriteFile(filepath.Join(agentsDir, agentName+".md"), []byte("version 2 content\n"), 0644); err != nil {
		t.Fatalf("failed to write updated agent file: %v", err)
	}
	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "update agent")

	// Run agents update for the specific agent
	out, _, code = env.run("agents", "update", agentName)
	if code != 0 {
		t.Fatalf("expected exit 0 from agents update, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, fmt.Sprintf("Updated agent %q", agentName)) {
		t.Errorf("expected 'Updated agent' in output, got:\n%s", out)
	}

	// Verify updated content
	content, err = os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed agent file after update: %v", err)
	}
	if string(content) != "version 2 content\n" {
		t.Fatalf("expected 'version 2 content\\n' after update, got %q", string(content))
	}
}

func TestAgentsUpdate_GitHub_AllAgents(t *testing.T) {
	env := newTestEnv(t)
	owner, repo, agentName := "test-owner-b", "test-repo-b", "all-update-agent"

	upstreamDir := makeGitUpstreamWithAgent(t, env, owner, repo, agentName, "v1\n")
	env.run("registry", "add", "github", owner+"/"+repo)
	env.run("agents", "add", agentName)

	// Push an update
	agentsDir := filepath.Join(upstreamDir, ".opencode", "agents")
	if err := os.WriteFile(filepath.Join(agentsDir, agentName+".md"), []byte("v2\n"), 0644); err != nil {
		t.Fatalf("failed to write updated agent: %v", err)
	}
	mustExec(t, upstreamDir, "git", "add", ".")
	mustExec(t, upstreamDir, "git", "commit", "-m", "update")

	// Run agents update with no args (updates all GitHub agents)
	out, _, code := env.run("agents", "update")
	if code != 0 {
		t.Fatalf("expected exit 0 from agents update, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, fmt.Sprintf("Updated agent %q", agentName)) {
		t.Errorf("expected 'Updated agent' in output, got:\n%s", out)
	}

	// Verify updated content
	installedPath := filepath.Join(env.projectDir, ".opencode", "agents", agentName+".md")
	content, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("failed to read installed agent file after update: %v", err)
	}
	if string(content) != "v2\n" {
		t.Fatalf("expected 'v2\\n' after update, got %q", string(content))
	}
}

// ---- Pack tests ----

func TestPackList_Empty(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("pack", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "No packs configured") {
		t.Errorf("expected 'No packs configured' in output, got:\n%s", out)
	}
}

func TestPackAdd(t *testing.T) {
	env := newTestEnv(t)
	out, _, code := env.run("pack", "add", "my-pack")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `Added pack "my-pack"`) {
		t.Errorf("expected 'Added pack' in output, got:\n%s", out)
	}
}

func TestPackAdd_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "add")
	if code == 0 {
		t.Fatal("expected non-zero exit when name arg is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestPackAdd_Duplicate(t *testing.T) {
	env := newTestEnv(t)
	env.run("pack", "add", "my-pack")
	_, errOut, code := env.run("pack", "add", "my-pack")
	if code == 0 {
		t.Fatal("expected non-zero exit when adding duplicate pack")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("expected 'already exists' in stderr, got:\n%s", errOut)
	}
}

func TestPackList_AfterAdd(t *testing.T) {
	env := newTestEnv(t)
	env.run("pack", "add", "alpha")
	env.run("pack", "add", "beta")

	out, _, code := env.run("pack", "list")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected 'alpha' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("expected 'beta' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "2 total") {
		t.Errorf("expected '2 total' in output, got:\n%s", out)
	}
}

func TestPackRemove(t *testing.T) {
	env := newTestEnv(t)
	env.run("pack", "add", "my-pack")

	out, _, code := env.run("pack", "remove", "my-pack")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, `Removed pack "my-pack"`) {
		t.Errorf("expected 'Removed pack' in output, got:\n%s", out)
	}

	// Verify it is gone
	listOut, _, _ := env.run("pack", "list")
	if !strings.Contains(listOut, "No packs configured") {
		t.Errorf("expected pack to be absent after removal, list output:\n%s", listOut)
	}
}

func TestPackRemove_NotFound(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "remove", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when removing nonexistent pack")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

func TestPackRemove_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "remove")
	if code == 0 {
		t.Fatal("expected non-zero exit when name arg is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestPackUpdate_NotFound(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "update", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when updating nonexistent pack")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

func TestPackUpdate_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "update")
	if code == 0 {
		t.Fatal("expected non-zero exit when name arg is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestPackInstall_NotFound(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "install", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit when installing nonexistent pack")
	}
	if !strings.Contains(errOut, "not found") {
		t.Errorf("expected 'not found' in stderr, got:\n%s", errOut)
	}
}

func TestPackInstall_MissingArgs(t *testing.T) {
	env := newTestEnv(t)
	_, errOut, code := env.run("pack", "install")
	if code == 0 {
		t.Fatal("expected non-zero exit when name arg is missing")
	}
	if !strings.Contains(errOut, "usage:") {
		t.Errorf("expected 'usage:' in stderr, got:\n%s", errOut)
	}
}

func TestPackInstall_EmptyPack(t *testing.T) {
	env := newTestEnv(t)
	env.run("pack", "add", "empty-pack")

	out, _, code := env.run("pack", "install", "empty-pack")
	if code != 0 {
		t.Fatalf("expected exit 0 installing empty pack, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, "installed") {
		t.Errorf("expected 'installed' summary in output, got:\n%s", out)
	}
}

func TestPackInstall_WithSkills(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "skill-a", "skill-b")
	env.run("registry", "add", "local", regDir)

	// Build a pack JSON with those skills and write it directly to the packs.json file
	// so we can test pack install without a TUI.
	packsFile := filepath.Join(env.xdgDataHome, "agent-manager", "packs.json")
	if err := os.MkdirAll(filepath.Dir(packsFile), 0755); err != nil {
		t.Fatalf("failed to create packs dir: %v", err)
	}
	packsJSON := fmt.Sprintf(`{"packs":[{"name":"skill-pack","skills":[{"name":"skill-a","registry":{"type":"local","location":%q}},{"name":"skill-b","registry":{"type":"local","location":%q}}],"agents":[]}]}`,
		regDir, regDir)
	if err := os.WriteFile(packsFile, []byte(packsJSON), 0644); err != nil {
		t.Fatalf("failed to write packs file: %v", err)
	}

	out, _, code := env.run("pack", "install", "skill-pack")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed skill "skill-a"`) {
		t.Errorf("expected 'Installed skill \"skill-a\"' in output, got:\n%s", out)
	}
	if !strings.Contains(out, `Installed skill "skill-b"`) {
		t.Errorf("expected 'Installed skill \"skill-b\"' in output, got:\n%s", out)
	}

	// Verify skills exist on disk
	for _, skillName := range []string{"skill-a", "skill-b"} {
		installedPath := filepath.Join(env.projectDir, ".opencode", "skills", skillName)
		if _, err := os.Lstat(installedPath); os.IsNotExist(err) {
			t.Errorf("expected skill %q to be installed at %s", skillName, installedPath)
		}
	}
}

func TestPackInstall_WithAgents(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistryWithAgents(t, "agent-a", "agent-b")
	env.run("registry", "add", "local", regDir)

	// Write a pack JSON with agents directly
	packsFile := filepath.Join(env.xdgDataHome, "agent-manager", "packs.json")
	if err := os.MkdirAll(filepath.Dir(packsFile), 0755); err != nil {
		t.Fatalf("failed to create packs dir: %v", err)
	}
	packsJSON := fmt.Sprintf(`{"packs":[{"name":"agent-pack","skills":[],"agents":[{"name":"agent-a","registry":{"type":"local","location":%q}},{"name":"agent-b","registry":{"type":"local","location":%q}}]}]}`,
		regDir, regDir)
	if err := os.WriteFile(packsFile, []byte(packsJSON), 0644); err != nil {
		t.Fatalf("failed to write packs file: %v", err)
	}

	out, _, code := env.run("pack", "install", "agent-pack")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed agent "agent-a"`) {
		t.Errorf("expected 'Installed agent \"agent-a\"' in output, got:\n%s", out)
	}
	if !strings.Contains(out, `Installed agent "agent-b"`) {
		t.Errorf("expected 'Installed agent \"agent-b\"' in output, got:\n%s", out)
	}

	// Verify agents exist on disk
	for _, agentName := range []string{"agent-a", "agent-b"} {
		installedPath := filepath.Join(env.projectDir, ".opencode", "agents", agentName+".md")
		if _, err := os.Lstat(installedPath); os.IsNotExist(err) {
			t.Errorf("expected agent %q to be installed at %s", agentName, installedPath)
		}
	}
}

func TestPackInstall_AlreadyInstalled(t *testing.T) {
	env := newTestEnv(t)
	regDir := makeLocalRegistry(t, "my-skill")
	env.run("registry", "add", "local", regDir)

	// Install the skill manually first
	env.run("skills", "add", "my-skill")

	// Write a pack that contains the already-installed skill
	packsFile := filepath.Join(env.xdgDataHome, "agent-manager", "packs.json")
	if err := os.MkdirAll(filepath.Dir(packsFile), 0755); err != nil {
		t.Fatalf("failed to create packs dir: %v", err)
	}
	packsJSON := fmt.Sprintf(`{"packs":[{"name":"my-pack","skills":[{"name":"my-skill","registry":{"type":"local","location":%q}}],"agents":[]}]}`,
		regDir)
	if err := os.WriteFile(packsFile, []byte(packsJSON), 0644); err != nil {
		t.Fatalf("failed to write packs file: %v", err)
	}

	out, _, code := env.run("pack", "install", "my-pack")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' for duplicate skill, got:\n%s", out)
	}
}
