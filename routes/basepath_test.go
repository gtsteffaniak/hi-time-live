package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNormalizeBasePath(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"/":         "",
		"hitime":    "/hitime",
		"/hitime":   "/hitime",
		"/hitime/":  "/hitime",
		" /a/b/ ":   "/a/b",
		"//hitime/": "/hitime",
	}
	for in, want := range cases {
		if got := NormalizeBasePath(in); got != want {
			t.Errorf("NormalizeBasePath(%q) = %q, want %q", in, got, want)
		}
	}
}

// newBasePathServer serves the real routes mounted under basePath.
func newBasePathServer(t *testing.T, basePath string) *httptest.Server {
	t.Helper()
	if err := os.Chdir(".."); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir("routes") })
	old := BasePath
	BasePath = NormalizeBasePath(basePath)
	t.Cleanup(func() { BasePath = old })
	templateRenderer = &TemplateRenderer{templateDir: "templates"}
	if err := templateRenderer.loadTemplates(); err != nil {
		t.Fatalf("could not load templates: %v", err)
	}
	server := httptest.NewServer(MountRouter(NewRouter(), BasePath))
	t.Cleanup(server.Close)
	return server
}

func get(t *testing.T, server *httptest.Server, path string) (*http.Response, string) {
	t.Helper()
	client := server.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return resp, string(body)
}

func TestMountRouterUnderBasePath(t *testing.T) {
	server := newBasePathServer(t, "/hitime")
	code := newCode(t)

	resp, body := get(t, server, "/hitime/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /hitime/ status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, `<base href="/hitime/">`) {
		t.Errorf("index page missing base tag: %.300s", body)
	}
	if strings.Contains(strings.Replace(body, `<base href="/hitime/">`, "", 1), `href="/`) || strings.Contains(body, `src="/`) {
		t.Errorf("index page still has root-absolute URLs that would escape the base path")
	}

	resp, body = get(t, server, "/hitime/room?id="+code)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "Privacy Notice") {
		t.Errorf("room page under base path: status %d, body %.200s", resp.StatusCode, body)
	}
	if strings.Contains(body, `fetch("/event"`) || strings.Contains(body, "EventSource(`/events") {
		t.Errorf("room scripts still use root-absolute API paths")
	}

	resp, _ = get(t, server, "/hitime/favicon-32x32.png")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("static asset under base path: status %d", resp.StatusCode)
	}

	resp, _ = get(t, server, "/hitime")
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/hitime/" {
		t.Errorf("bare prefix: status %d location %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	for _, path := range []string{"/", "/room?id=" + code, "/events?userId=u&code=" + code} {
		resp, _ = get(t, server, path)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s outside base path: status %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestRootBasePathKeepsRootRelativeBase(t *testing.T) {
	server := newBasePathServer(t, "")
	resp, body := get(t, server, "/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d", resp.StatusCode)
	}
	if !strings.Contains(body, `<base href="/">`) {
		t.Errorf("index page at root missing base tag: %.300s", body)
	}
}
