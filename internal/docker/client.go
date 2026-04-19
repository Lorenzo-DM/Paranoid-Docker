package docker

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
)

func NewClient() (*client.Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	if host := os.Getenv("DOCKER_HOST"); host != "" {
		opts = append(opts, client.WithHost(host))
	} else if sock := detectDockerSocket(); sock != "" {
		opts = append(opts, client.WithHost("unix://"+sock))
	}

	return client.NewClientWithOpts(opts...)
}

func detectDockerSocket() string {
	candidates := []string{
		"/var/run/docker.sock",
		filepath.Join(os.Getenv("HOME"), ".docker", "run", "docker.sock"),
	}
	for _, s := range candidates {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	return ""
}
