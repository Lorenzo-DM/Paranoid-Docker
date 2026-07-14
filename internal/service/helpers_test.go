package service

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/model"

	"github.com/docker/docker/api/types"
)

func TestPinImageDigest(t *testing.T) {
	cases := []struct {
		name   string
		image  string
		digest string
		want   string
	}{
		{"tag replaced", "nginx:1.25", "sha256:abc", "nginx@sha256:abc"},
		{"no tag", "nginx", "sha256:abc", "nginx@sha256:abc"},
		{"existing digest replaced", "nginx@sha256:old", "sha256:new", "nginx@sha256:new"},
		{"registry with port keeps host", "reg.local:5000/app:v1", "sha256:abc", "reg.local:5000/app@sha256:abc"},
		{"empty digest returns image", "nginx:1.25", "", "nginx:1.25"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pinImageDigest(tc.image, tc.digest); got != tc.want {
				t.Errorf("pinImageDigest(%q, %q) = %q, want %q", tc.image, tc.digest, got, tc.want)
			}
		})
	}
}

func TestBuildPinnedImages(t *testing.T) {
	stack := model.ComposeStack{
		Services: []model.ComposeService{
			{Name: "web", ContainerID: "c1", Image: "nginx:1.25", LocalDigest: "sha256:abc"},
			{Name: "db", ContainerID: "c2", Image: "postgres:16", LocalDigest: ""},
			{Name: "gone", ContainerID: "", Image: "redis:7", LocalDigest: "sha256:def"},
		},
	}
	pinned := buildPinnedImages(stack)
	if len(pinned) != 1 {
		t.Fatalf("expected 1 pinned image, got %d: %v", len(pinned), pinned)
	}
	if pinned["web"] != "nginx@sha256:abc" {
		t.Errorf("web = %q, want nginx@sha256:abc", pinned["web"])
	}
}

func TestPatchComposeImages(t *testing.T) {
	src := `services:
  web:
    image: nginx:1.25
    ports:
      - "80:80"
  db:
    image: postgres:16
`
	out, err := patchComposeImages([]byte(src), map[string]string{"web": "nginx@sha256:abc"})
	if err != nil {
		t.Fatalf("patchComposeImages: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "nginx@sha256:abc") {
		t.Errorf("patched output missing pinned image:\n%s", s)
	}
	if !strings.Contains(s, "postgres:16") {
		t.Errorf("unpinned service image changed:\n%s", s)
	}
}

func TestRestartPolicyName(t *testing.T) {
	for input, want := range map[string]string{
		"always":         "always",
		"unless-stopped": "unless-stopped",
		"on-failure":     "on-failure",
		"no":             "",
		"":               "",
	} {
		if got := restartPolicyName(input); got != want {
			t.Errorf("restartPolicyName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEffectiveMode(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "docker-compose.yaml")
	if err := os.WriteFile(existing, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		mode        string
		configFiles []string
		want        string
	}{
		{"forced compose", "compose", nil, "compose"},
		{"forced inspect", "inspect", []string{existing}, "inspect"},
		{"auto with accessible file", "auto", []string{existing}, "compose"},
		{"auto with missing file", "auto", []string{filepath.Join(tmp, "missing.yaml")}, "inspect"},
		{"auto with no files", "auto", nil, "inspect"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &composeStackService{rollbackMode: tc.mode}
			if got := svc.effectiveMode(tc.configFiles); got != tc.want {
				t.Errorf("effectiveMode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilenameToImageRef(t *testing.T) {
	for input, want := range map[string]string{
		"nginx_1.25_abcd1234.tar.gz":       "nginx_1.25",
		"library_nginx_latest_ab12.tar.gz": "library_nginx_latest",
		"plain.tar.gz":                     "plain",
	} {
		if got := filenameToImageRef(input); got != want {
			t.Errorf("filenameToImageRef(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStackStatus(t *testing.T) {
	cases := []struct {
		running, total int
		want           string
	}{
		{0, 3, "stopped"},
		{3, 3, "running"},
		{1, 3, "partial"},
	}
	for _, tc := range cases {
		if got := stackStatus(tc.running, tc.total); got != tc.want {
			t.Errorf("stackStatus(%d, %d) = %q, want %q", tc.running, tc.total, got, tc.want)
		}
	}
}

func TestMapContainer(t *testing.T) {
	c := types.Container{
		ID:      "0123456789abcdef",
		Names:   []string{"/myapp"},
		Image:   "nginx:1.25",
		ImageID: "sha256:img",
		State:   "running",
		Status:  "Up 2 hours",
		Created: 1700000000,
		Labels:  map[string]string{"com.docker.compose.project": "stack1"},
		Ports:   []types.Port{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
	}
	mc := mapContainer(c)
	if mc.Name != "myapp" {
		t.Errorf("Name = %q, want myapp", mc.Name)
	}
	if mc.ShortID != "0123456789ab" {
		t.Errorf("ShortID = %q, want 0123456789ab", mc.ShortID)
	}
	if !mc.ComposeManagedFilter {
		t.Error("expected ComposeManagedFilter true for compose-labeled container")
	}
	if len(mc.Ports) != 1 || mc.Ports[0].HostPort != "8080" || mc.Ports[0].ContainerPort != "80" {
		t.Errorf("Ports = %+v", mc.Ports)
	}
}

func TestParsePullStream(t *testing.T) {
	t.Run("progress events", func(t *testing.T) {
		body := `{"status":"Pulling from library/nginx","id":"latest"}
{"status":"Downloading","progressDetail":{"current":10,"total":100},"progress":"[=>   ]","id":"layer1"}
{"status":"Pull complete","id":"layer1"}
`
		ch := make(chan PullEvent, 16)
		ParsePullStream(io.NopCloser(strings.NewReader(body)), ch)

		var events []PullEvent
		for evt := range ch {
			events = append(events, evt)
		}
		if len(events) != 3 {
			t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
		}
		if events[1].Progress != "[=>   ]" || events[1].ID != "layer1" {
			t.Errorf("event 1 = %+v", events[1])
		}
	})

	t.Run("error stops stream and closes channel", func(t *testing.T) {
		body := `{"status":"Pulling"}
{"error":"manifest unknown"}
{"status":"never emitted"}
`
		ch := make(chan PullEvent, 16)
		ParsePullStream(io.NopCloser(strings.NewReader(body)), ch)

		var events []PullEvent
		for evt := range ch {
			events = append(events, evt)
		}
		if len(events) != 2 {
			t.Fatalf("expected 2 events, got %d: %+v", len(events), events)
		}
		last := events[len(events)-1]
		if last.Type != "error" || last.Error != "manifest unknown" {
			t.Errorf("last event = %+v, want error", last)
		}
	})

	t.Run("malformed lines skipped", func(t *testing.T) {
		body := "not json\n{\"status\":\"ok\"}\n"
		ch := make(chan PullEvent, 16)
		ParsePullStream(io.NopCloser(strings.NewReader(body)), ch)

		var events []PullEvent
		for evt := range ch {
			events = append(events, evt)
		}
		if len(events) != 1 || events[0].Status != "ok" {
			t.Errorf("events = %+v", events)
		}
	})
}

func TestLineWriter(t *testing.T) {
	var lines []string
	w := &lineWriter{emit: func(l string) { lines = append(lines, l) }}
	_, _ = w.Write([]byte("first li"))
	_, _ = w.Write([]byte("ne\nsecond line\npart"))
	w.Flush()
	want := []string{"first line", "second line", "part"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}
