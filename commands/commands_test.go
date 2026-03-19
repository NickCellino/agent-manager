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

// addGitHubRegistryOrSkip adds the NickCellino/laptop-setup GitHub registry to env,
// then runs `skills list` to trigger a clone of the repo.  If the "playground" skill
// does not appear in the output (e.g. the repo is private or network is unavailable),
// the test is skipped gracefully.  On success it returns the `skills list` stdout so
// the caller can make additional assertions without running the command a second time.
func addGitHubRegistryOrSkip(t *testing.T, env *testEnv) string {
	t.Helper()
	env.run("registry", "add", "github", "NickCellino/laptop-setup")
	out, errOut, _ := env.run("skills", "list")
	if !strings.Contains(out, "playground") {
		t.Skipf("NickCellino/laptop-setup not accessible or 'playground' skill not found; skipping GitHub registry test.\nstdout: %s\nstderr: %s", out, errOut)
	}
	return out
}

func TestRegistryAdd_GitHub(t *testing.T) {
	env := newTestEnv(t)

	out, _, code := env.run("registry", "add", "github", "NickCellino/laptop-setup")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(out, "Added github registry") {
		t.Errorf("expected 'Added github registry' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "NickCellino/laptop-setup") {
		t.Errorf("expected 'NickCellino/laptop-setup' in output, got:\n%s", out)
	}

	// Verify it appears in the list (no network needed — registry config is local)
	listOut, _, _ := env.run("registry", "list")
	if !strings.Contains(listOut, "[github]") {
		t.Errorf("expected '[github]' in registry list output, got:\n%s", listOut)
	}
	if !strings.Contains(listOut, "NickCellino/laptop-setup") {
		t.Errorf("expected 'NickCellino/laptop-setup' in registry list output, got:\n%s", listOut)
	}
}

func TestSkillsList_GitHub(t *testing.T) {
	env := newTestEnv(t)
	out := addGitHubRegistryOrSkip(t, env)

	// addGitHubRegistryOrSkip already verified "playground" is present; also check
	// that the registry type and location are shown for that skill.
	if !strings.Contains(out, "[github: NickCellino/laptop-setup]") {
		t.Errorf("expected '[github: NickCellino/laptop-setup]' in skills list output, got:\n%s", out)
	}
}

func TestSkillsAdd_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistryOrSkip(t, env)

	out, _, code := env.run("skills", "add", "playground")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Installed skill "playground"`) {
		t.Errorf("expected 'Installed skill \"playground\"' in output, got:\n%s", out)
	}

	// GitHub registries copy the skill directory (not a symlink)
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "playground")
	info, err := os.Lstat(installedPath)
	if err != nil {
		t.Fatalf("expected playground skill to exist at %s: %v", installedPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected a regular directory (not symlink) for a GitHub registry skill at %s", installedPath)
	}
}

func TestSkillsRemove_GitHub(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistryOrSkip(t, env)

	env.run("skills", "add", "playground")

	out, _, code := env.run("skills", "remove", "playground")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout: %s", code, out)
	}
	if !strings.Contains(out, `Removed skill "playground"`) {
		t.Errorf("expected 'Removed skill \"playground\"' in output, got:\n%s", out)
	}

	// Verify the skill directory was removed from the filesystem
	installedPath := filepath.Join(env.projectDir, ".opencode", "skills", "playground")
	if _, err := os.Lstat(installedPath); !os.IsNotExist(err) {
		t.Errorf("expected playground skill to be absent at %s after removal", installedPath)
	}

	// Verify it's no longer in the installed list
	installedOut, _, _ := env.run("skills", "installed")
	if strings.Contains(installedOut, "playground") {
		t.Errorf("expected 'playground' to be absent from installed list after removal, got:\n%s", installedOut)
	}
}

func TestFullGitHubRegistryWorkflow(t *testing.T) {
	env := newTestEnv(t)
	addGitHubRegistryOrSkip(t, env)

	// Install the playground skill
	env.run("skills", "add", "playground")

	// Verify it appears in the installed list
	installedOut, _, code := env.run("skills", "installed")
	if code != 0 {
		t.Fatalf("skills installed: expected exit 0, got %d", code)
	}
	if !strings.Contains(installedOut, "playground") {
		t.Errorf("expected 'playground' in installed list, got:\n%s", installedOut)
	}
	if !strings.Contains(installedOut, "[github: NickCellino/laptop-setup]") {
		t.Errorf("expected registry info in installed list, got:\n%s", installedOut)
	}

	// Installing again should be idempotent
	idempotentOut, _, idempotentCode := env.run("skills", "add", "playground")
	if idempotentCode != 0 {
		t.Fatalf("expected exit 0 on duplicate install, got %d", idempotentCode)
	}
	if !strings.Contains(idempotentOut, "already installed") {
		t.Errorf("expected 'already installed' on duplicate install, got:\n%s", idempotentOut)
	}

	// Remove the skill
	env.run("skills", "remove", "playground")

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
