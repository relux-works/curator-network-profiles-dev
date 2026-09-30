package main_test

// The N-A milestone test: two launches of the built provider use two
// loopback egresses at the same time, with hostile ambient proxy
// variables in the parent, and refusals happen before the child runs.
//
// Everything is loopback: three in-process proxies on 127.0.0.1, a
// temporary HOME, no DNS name resolved (the client asks the proxy for
// http://egress.invalid/ and the proxy answers itself). The
// confirmation ledger is seeded as a FIXTURE: `curator network confirm`
// refuses inside an agent session or without a terminal by design, so
// the fixture stands in for the operator's confirmation.

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/relux-works/curator-network-profiles/internal/confirm"
	"github.com/relux-works/curator-network-profiles/internal/testproxy"
	"github.com/relux-works/curator-network-profiles/pkg/netprofile"
)

var binDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "curator-network-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binDir = dir
	// The Go tool needs its own cache and module state; it inherits the
	// test process environment. The built binaries never see it: every
	// launch below carries an explicit environment with a temporary HOME.
	for _, b := range []struct{ name, pkg string }{{"curator-network", "."}, {"egress-client", "../../internal/testproxy/cmd/egress-client"}} {
		cmd := exec.Command("go", "build", "-o", filepath.Join(dir, b.name), b.pkg)
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n%s", b.name, err, out)
			_ = os.RemoveAll(dir)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type fixture struct {
	home  string
	env   []string
	a, b  *testproxy.Proxy
	wrong *testproxy.Proxy
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{home: t.TempDir()}
	for _, p := range []struct {
		id  string
		dst **testproxy.Proxy
	}{{"A", &f.a}, {"B", &f.b}, {"WRONG", &f.wrong}} {
		px, err := testproxy.Start(testproxy.Options{ID: p.id})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = px.Close() })
		*p.dst = px
	}
	base := []string{"HOME=" + f.home, "PATH=" + binDir, "XDG_CONFIG_HOME=" + filepath.Join(f.home, "xdg"), "TMPDIR=" + f.home}
	for _, add := range []struct{ name, url string }{{"egress-a", f.a.URL()}, {"egress-b", f.b.URL()}} {
		cmd := exec.Command(filepath.Join(binDir, "curator-network"), "add", add.name, "--endpoint", add.url)
		cmd.Env = base
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("add %s: %v\n%s", add.name, err, out)
		}
	}
	// A profile whose proxy is not listening, for the refusal case.
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := "http://" + closed.Addr().String()
	_ = closed.Close()
	cmd := exec.Command(filepath.Join(binDir, "curator-network"), "add", "egress-dead", "--endpoint", dead)
	cmd.Env = base
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add egress-dead: %v\n%s", err, out)
	}
	// FIXTURE: confirm every profile at its current digest.
	data, err := os.ReadFile(filepath.Join(f.home, ".curator", "network.toml"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := netprofile.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	ledger := confirm.Empty()
	for _, name := range file.Names() {
		p, _ := file.Lookup(name)
		ledger.Set(name, netprofile.Digest(p), time.Now())
	}
	if err := confirm.Write(filepath.Join(f.home, ".curator", "network.confirmations.json"), ledger); err != nil {
		t.Fatal(err)
	}
	// Hostile ambient values in the parent of every launch.
	f.env = append(base,
		"HTTPS_PROXY="+f.wrong.URL(), "http_proxy="+f.wrong.URL(), "all_proxy="+f.wrong.URL(),
		"NO_PROXY=*", "Http_Proxy="+f.wrong.URL())
	return f
}

type launch struct {
	stdout, stderr string
	code           int
}

func (f *fixture) launch(t *testing.T, args ...string) launch {
	t.Helper()
	cmd := exec.Command(filepath.Join(binDir, "curator-network"), args...)
	cmd.Env = f.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errorsAs(err, &ee) {
			t.Fatalf("run %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return launch{stdout.String(), stderr.String(), code}
}

func errorsAs(err error, target **exec.ExitError) bool {
	ee, ok := err.(*exec.ExitError)
	if ok {
		*target = ee
	}
	return ok
}

func TestTwoEgressesConcurrently(t *testing.T) {
	f := newFixture(t)
	before := os.Environ()
	var wg sync.WaitGroup
	var la, lb launch
	wg.Add(2)
	go func() { defer wg.Done(); la = f.launch(t, "exec", "egress-a", "--", "egress-client") }()
	go func() { defer wg.Done(); lb = f.launch(t, "exec", "egress-b", "--", "egress-client") }()
	wg.Wait()
	if la.code != 0 || la.stdout != "egress=A\n" {
		t.Errorf("launch A: code %d stdout %q stderr %q", la.code, la.stdout, la.stderr)
	}
	if lb.code != 0 || lb.stdout != "egress=B\n" {
		t.Errorf("launch B: code %d stdout %q stderr %q", lb.code, lb.stdout, lb.stderr)
	}
	for _, l := range []launch{la, lb} {
		if !strings.HasPrefix(l.stderr, "curator-network: network=egress-") || !strings.Contains(l.stderr, "preflight=tcp ok, connect skipped, tls skipped") {
			t.Errorf("launch stderr %q", l.stderr)
		}
	}
	want := []testproxy.Request{{Method: "GET", Target: "http://egress.invalid/"}}
	if got := f.a.Requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("proxy A saw %+v", got)
	}
	if got := f.b.Requests(); !reflect.DeepEqual(got, want) {
		t.Errorf("proxy B saw %+v", got)
	}
	if got := f.wrong.Requests(); len(got) != 0 {
		t.Errorf("the ambient proxy saw %+v", got)
	}
	if after := os.Environ(); !reflect.DeepEqual(before, after) {
		t.Error("the parent environment changed")
	}
	if strings.Join(f.env, "\n") != strings.Join(append([]string{}, f.env...), "\n") || !strings.Contains(strings.Join(f.env, "\n"), "NO_PROXY=*") {
		t.Error("the launch environment slice was altered")
	}
}

func TestRefusalsHappenBeforeTheChildRuns(t *testing.T) {
	f := newFixture(t)
	cases := []struct {
		name    string
		profile string
		code    string
	}{
		{"unknown profile", "egress-zz", "network_profile_unknown"},
		{"unreachable proxy", "egress-dead", "network_proxy_unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(f.home, tc.profile+".marker")
			l := f.launch(t, "exec", tc.profile, "--preflight-timeout", "500ms", "--", "egress-client", "--marker", marker)
			if l.code != 1 || !strings.HasPrefix(l.stderr, "curator-network: "+tc.code+": "+tc.profile+": ") || l.stdout != "" {
				t.Fatalf("code %d stdout %q stderr %q", l.code, l.stdout, l.stderr)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("the child ran: marker exists")
			}
		})
	}
	if got := f.wrong.Requests(); len(got) != 0 {
		t.Errorf("the ambient proxy saw %+v", got)
	}
	// The client itself never dials directly: without a proxy it exits 3.
	cmd := exec.Command(filepath.Join(binDir, "egress-client"))
	cmd.Env = []string{"HOME=" + f.home, "PATH=" + binDir}
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errorsAs(err, &ee) || ee.ExitCode() != 3 || !strings.Contains(string(out), "refusing to dial directly") {
		t.Fatalf("egress-client without a proxy: %v %s", err, out)
	}
}

func TestDryRunAndJSONThroughTheBinary(t *testing.T) {
	f := newFixture(t)
	l := f.launch(t, "--json", "exec", "egress-a", "--dry-run", "--", "egress-client")
	if l.code != 0 || !strings.Contains(l.stdout, `"schema":"curator-network-exec-v1"`) || !strings.Contains(l.stdout, `"profile_ref":"egress-a"`) {
		t.Fatalf("dry run: code %d stdout %q stderr %q", l.code, l.stdout, l.stderr)
	}
	if got := f.a.Requests(); len(got) != 0 {
		t.Fatalf("dry run reached the proxy: %+v", got)
	}
	l = f.launch(t, "check", "--all", "--probe", "--json")
	if l.code != 1 || !strings.Contains(l.stdout, `"schema":"curator-network-check-v1","ok":false`) || !strings.Contains(l.stderr, "network_proxy_unreachable: egress-dead") {
		t.Fatalf("check: code %d stdout %q stderr %q", l.code, l.stdout, l.stderr)
	}
	l = f.launch(t, "list")
	if l.code != 0 || !strings.Contains(l.stdout, "egress-a") || !strings.Contains(l.stdout, "yes") {
		t.Fatalf("list: %q", l.stdout)
	}
}
