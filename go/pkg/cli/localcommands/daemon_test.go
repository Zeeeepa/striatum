package localcommands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDaemonSubcommandsAreLocal(t *testing.T) {
	for _, sub := range []string{"install", "uninstall", "status"} {
		if _, ok := Lookup([]string{"daemon", sub}); !ok {
			t.Fatalf("daemon %s not registered as a local command", sub)
		}
	}
}

func TestRenderUnitUsesSpecifiersNotHardcodedHome(t *testing.T) {
	unit := renderUnit()
	if !strings.Contains(unit, "%h/.local/bin/striatumd") {
		t.Fatalf("unit missing %%h-relative ExecStart:\n%s", unit)
	}
	if !strings.Contains(unit, "%t/striatum/daemon-go.sock") {
		t.Fatalf("unit missing %%t runtime socket:\n%s", unit)
	}
	if strings.Contains(unit, "striatumd.sock") {
		t.Fatalf("unit references retired striatumd.sock:\n%s", unit)
	}
	if home, _ := os.UserHomeDir(); home != "" && strings.Contains(unit, home) {
		t.Fatalf("unit contains hardcoded home path %q:\n%s", home, unit)
	}
}

func TestDaemonInstallPrintUnit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := RunDaemon([]string{"install", "--print-unit"}, &stdout, &stderr, "test"); code != 0 {
		t.Fatalf("print-unit exit=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ExecStart=%h/.local/bin/striatumd") {
		t.Fatalf("print-unit output missing ExecStart:\n%s", stdout.String())
	}
}

func TestScaffoldDaemonTOMLNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.toml")

	created, err := scaffoldDaemonTOML(path)
	if err != nil {
		t.Fatalf("scaffold: %v", err)
	}
	if !created {
		t.Fatal("expected scaffold to create the file")
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "postgres_url") {
		t.Fatalf("scaffold missing postgres_url guidance:\n%s", body)
	}

	// Simulate a real config and confirm a second call leaves it untouched.
	real := "postgres_url = \"postgres://real@localhost/striatum\"\n"
	if err := os.WriteFile(path, []byte(real), 0o600); err != nil {
		t.Fatal(err)
	}
	created, err = scaffoldDaemonTOML(path)
	if err != nil {
		t.Fatalf("scaffold (existing): %v", err)
	}
	if created {
		t.Fatal("scaffold overwrote an existing config")
	}
	body, _ = os.ReadFile(path)
	if string(body) != real {
		t.Fatalf("scaffold mutated existing config:\n%s", body)
	}
}

func TestResolveLayoutUsesCanonicalSocket(t *testing.T) {
	runtimeDir := t.TempDir()
	configHomeDir := t.TempDir()
	t.Setenv("STRIATUM_DAEMON_RUNTIME_DIR", runtimeDir)
	t.Setenv("XDG_CONFIG_HOME", configHomeDir)

	l, err := resolveLayout()
	if err != nil {
		t.Fatalf("resolveLayout: %v", err)
	}
	if !strings.HasSuffix(l.socket, "daemon-go.sock") {
		t.Fatalf("socket not canonical: %s", l.socket)
	}
	if strings.Contains(l.socket, "striatumd.sock") {
		t.Fatalf("socket references retired name: %s", l.socket)
	}
	wantUnit := filepath.Join(configHomeDir, "systemd", "user", "striatumd.service")
	if l.unitPath != wantUnit {
		t.Fatalf("unit path = %s, want %s", l.unitPath, wantUnit)
	}
}
