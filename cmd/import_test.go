package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const envFile = `# secrets
export NEW_TOKEN='new-token-value'
export PEM="-----BEGIN-----
pem-body
-----END-----"
export API_TOKEN=replaced-value
export URL="https://user:$USED_TOKEN@host"
USED_TOKEN=used-token-value
lower=lower-value
export EMPTY=
DUP=first-dup-value
DUP="second dup value"
export CMD=$(pass show cmd-value)
export LATE=early-value
export LATE="$HOME/late"
alias ll='ls -l'
`

// envValues are every value in envFile, none of which may be printed or
// logged.
var envValues = []string{"new-token-value", "pem-body", "replaced-value", "used-token-value",
	"lower-value", "first-dup-value", "second dup value", "cmd-value", "early-value"}

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "zshenv")
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func assertNoEnvValues(t *testing.T, outputs ...string) {
	t.Helper()
	for _, out := range outputs {
		for _, v := range envValues {
			if strings.Contains(out, v) {
				t.Errorf("output holds the value %q:\n%s", v, out)
			}
		}
	}
}

func TestImport(t *testing.T) {
	seedVault(t)
	home := os.Getenv("BROKER_HOME")
	file := writeEnv(t, envFile)

	out, stderr := mustRun(t, "import", file)
	want := `imported (4):
  NEW_TOKEN   private  line 2
  PEM         private  lines 3-5
  USED_TOKEN  private  line 8
  DUP         private  lines 11-12
skipped (7):
  line 6   API_TOKEN  already in the vault (--overwrite replaces it)
  line 7   URL        variable reference
  line 9   lower      invalid name: must be A-Z, 0-9, and _, not starting with a digit
  line 10  EMPTY      value is empty
  line 13  CMD        command substitution
  line 14  LATE       set again on line 15
  line 15  LATE       variable reference
kept, still used in FILE (1):
  line 8  USED_TOKEN  used on line 7
safe to delete from FILE: lines 2-5, 11-12
`
	if got := strings.ReplaceAll(out, file, "FILE"); got != want || stderr != "" {
		t.Fatalf("stdout:\n%s\nwant:\n%s\nstderr %q", got, want, stderr)
	}
	assertNoEnvValues(t, out, stderr)

	for name, value := range map[string]string{
		"NEW_TOKEN":  "new-token-value",
		"PEM":        "-----BEGIN-----\npem-body\n-----END-----",
		"USED_TOKEN": "used-token-value",
		"DUP":        "second dup value",
		"API_TOKEN":  "token-value",
	} {
		if got, _ := mustRun(t, "get", name); got != value+"\n" {
			t.Errorf("get %s = %q, want %q", name, got, value)
		}
	}
	for _, name := range []string{"URL", "lower", "EMPTY", "CMD", "LATE"} {
		if _, _, code := runStdin(t, "", "get", name); code == 0 {
			t.Errorf("%s was imported", name)
		}
	}

	// --overwrite replaces existing secrets; --public moves them.
	out, _ = mustRun(t, "import", file, "--overwrite", "--public")
	if !strings.Contains(out, "  API_TOKEN   public  line 6\n") ||
		!strings.HasSuffix(out, "safe to delete from "+file+": lines 2-6, 11-12\n") {
		t.Fatalf("overwrite stdout:\n%s", out)
	}
	assertNoEnvValues(t, out)
	if got, _ := mustRun(t, "get", "API_TOKEN"); got != "replaced-value\n" {
		t.Errorf("get API_TOKEN = %q after --overwrite", got)
	}
	for _, s := range ls(t) {
		if s.Tier != "public" && s.Name != "DB_PASS" {
			t.Errorf("%s is %s after --public", s.Name, s.Tier)
		}
	}

	got := summaries(auditLog(t, "--since", "1h"))
	got = slices.DeleteFunc(got, func(s string) bool { return !strings.HasPrefix(s, "import ") })
	wantLog := []string{
		"import NEW_TOKEN,PEM,USED_TOKEN,DUP private ok",
		"import NEW_TOKEN,PEM,API_TOKEN,USED_TOKEN,DUP public ok",
	}
	if !slices.Equal(got, wantLog) {
		t.Errorf("log = %q, want %q", got, wantLog)
	}
	assertNoValues(t, home, envValues...)
}

func TestImportDryRun(t *testing.T) {
	seedVault(t)
	file := writeEnv(t, "export A='a-value'\nexport API_USER=x\n")
	before := ls(t)

	out, stderr := mustRun(t, "import", file, "--dry-run", "--overwrite")
	want := `would import (2):
  A         private  line 1
  API_USER  public   line 2
after import, safe to delete from FILE: lines 1-2
`
	if got := strings.ReplaceAll(out, file, "FILE"); got != want || stderr != "" {
		t.Fatalf("stdout:\n%s\nwant:\n%s\nstderr %q", got, want, stderr)
	}
	if after := ls(t); !slices.Equal(after, before) {
		t.Errorf("dry run changed the vault: %+v", after)
	}
	for _, s := range summaries(auditLog(t)) {
		if strings.HasPrefix(s, "import") {
			t.Errorf("dry run logged %q", s)
		}
	}
}

func TestImportNothing(t *testing.T) {
	initVault(t)
	file := writeEnv(t, "# nothing here\nexport PATH=\"$HOME/bin:$PATH\"\n")
	out, _ := mustRun(t, "import", file)
	want := "nothing imported from FILE\nskipped (1):\n  line 2  PATH  variable reference\n"
	if got := strings.ReplaceAll(out, file, "FILE"); got != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", got, want)
	}
	if got := summaries(auditLog(t)); !slices.Contains(got, "import   ok") {
		t.Errorf("log = %q", got)
	}
}

func TestImportErrors(t *testing.T) {
	file := writeEnv(t, "A=1\n")
	_, stderr, code := runBroker(t, "import", file)
	if code != 66 || errorCode(t, stderr) != "no_vault" {
		t.Fatalf("no vault: exit %d %s", code, stderr)
	}

	initVault(t)
	_, stderr, code = runStdin(t, "", "import", filepath.Join(t.TempDir(), "missing"))
	if code != 1 || errorCode(t, stderr) != "error" {
		t.Fatalf("missing file: exit %d %s", code, stderr)
	}
	if got := summaries(auditLog(t)); !slices.Contains(got, "import   error") {
		t.Errorf("log = %q", got)
	}
}
