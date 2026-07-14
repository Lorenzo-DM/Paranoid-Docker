package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repository"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
)

type ContainerService interface {
	GetAll(ctx context.Context) ([]model.Container, error)
	UpdateContainer(ctx context.Context, id string, progressCh chan<- PullEvent, includeEnv bool) error
	StreamLogs(ctx context.Context, id string, w io.Writer) error
	ListRollbacksForContainer(containerName string) ([]model.RollbackFile, error)
}

type containerService struct {
	repo          repository.ContainerRepository
	digestChecker DigestChecker
	rollback      *RollbackWriter
}

func NewContainerService(repo repository.ContainerRepository, dc DigestChecker, rw *RollbackWriter) ContainerService {
	return &containerService{repo: repo, digestChecker: dc, rollback: rw}
}

func (s *containerService) GetAll(ctx context.Context) ([]model.Container, error) {
	dockerContainers, err := s.repo.ListContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]model.Container, 0, len(dockerContainers))
	for _, c := range dockerContainers {
		mc := mapContainer(c)

		localDigest, remoteDigest, updateAvailable := s.checkUpdate(ctx, c)
		mc.LocalDigest = localDigest
		mc.RemoteDigest = remoteDigest
		mc.UpdateAvailable = updateAvailable

		result = append(result, mc)
	}
	return result, nil
}

func (s *containerService) checkUpdate(ctx context.Context, c types.Container) (localDigest, remoteDigest string, updateAvailable bool) {
	imageRef := c.Image
	if strings.Contains(imageRef, "@sha256:") {
		return "", "", false
	}

	inspect, err := s.repo.InspectImage(ctx, c.ImageID)
	if err != nil {
		return "", "", false
	}
	for _, rd := range inspect.RepoDigests {
		if idx := strings.Index(rd, "@"); idx != -1 {
			localDigest = rd[idx+1:]
			break
		}
	}

	if localDigest == "" {
		return "", "", false
	}

	remoteDigest, err = s.digestChecker.GetLocalTagDigest(ctx, imageRef)
	if err != nil {
		return localDigest, "", false
	}

	return localDigest, remoteDigest, localDigest != remoteDigest
}

func (s *containerService) UpdateContainer(ctx context.Context, id string, progressCh chan<- PullEvent, includeEnv bool) error {
	defer close(progressCh)

	inspect, err := s.repo.InspectContainer(ctx, id)
	if err != nil {
		progressCh <- PullEvent{Type: "error", Error: fmt.Sprintf("inspect: %s", err)}
		return err
	}

	cfg := captureContainerConfig(inspect)
	captureNetworkDefs(ctx, s.repo, &cfg)

	localDigest := ""
	imgInspect, err := s.repo.InspectImage(ctx, inspect.Image)
	if err == nil {
		for _, rd := range imgInspect.RepoDigests {
			if _, after, ok := strings.Cut(rd, "@"); ok {
				localDigest = after
				break
			}
		}
	}

	pinnedRef := cfg.Image
	if localDigest != "" {
		if idx := strings.Index(cfg.Image, ":"); idx != -1 {
			pinnedRef = cfg.Image[:idx] + "@" + localDigest
		} else {
			pinnedRef = cfg.Image + "@" + localDigest
		}
	}

	if _, ok := inspect.Config.Labels["com.docker.compose.project"]; ok {
		progressCh <- PullEvent{
			Type:    "progress",
			Status:  "WARNING: this container is managed by Docker Compose — recreation may conflict",
			Message: "warning",
		}
	}

	progressCh <- PullEvent{Type: "progress", Status: "Generating rollback compose file..."}
	if _, err := s.rollback.WriteRollbackCompose(cfg, pinnedRef, includeEnv); err != nil {
		progressCh <- PullEvent{Type: "progress", Status: fmt.Sprintf("WARNING: could not write rollback file: %s", err)}
	} else {
		progressCh <- PullEvent{Type: "progress", Status: "Rollback compose file saved"}
	}

	progressCh <- PullEvent{Type: "progress", Status: fmt.Sprintf("Pulling %s...", cfg.Image)}
	pullStream, err := s.repo.PullImage(ctx, cfg.Image)
	if err != nil {
		progressCh <- PullEvent{Type: "error", Error: fmt.Sprintf("pull: %s", err)}
		return err
	}

	pullCh := make(chan PullEvent, 64)
	go ParsePullStream(pullStream, pullCh)
	for evt := range pullCh {
		progressCh <- evt
		if evt.Type == "error" {
			return fmt.Errorf("pull error: %s", evt.Error)
		}
	}

	newID, err := recreateContainer(ctx, s.repo, cfg, func(line string) {
		progressCh <- PullEvent{Type: "progress", Status: line}
	})
	if err != nil {
		progressCh <- PullEvent{Type: "error", Error: err.Error()}
		return err
	}

	progressCh <- PullEvent{
		Type:    "done",
		Status:  "Container updated successfully",
		Message: newID,
	}
	return nil
}

func (s *containerService) StreamLogs(ctx context.Context, id string, w io.Writer) error {
	rc, err := s.repo.ContainerLogs(ctx, id, true)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = stdcopy.StdCopy(w, w, rc)
	return err
}

func (s *containerService) ListRollbacksForContainer(containerName string) ([]model.RollbackFile, error) {
	return s.rollback.ListRollbacks(containerName)
}

func mapContainer(c types.Container) model.Container {
	name := ""
	if len(c.Names) > 0 {
		name = strings.TrimPrefix(c.Names[0], "/")
	}
	isCompose := c.Labels["com.docker.compose.project"] != ""

	ports := make([]model.Port, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, model.Port{
			HostPort:      fmt.Sprintf("%d", p.PublicPort),
			ContainerPort: fmt.Sprintf("%d", p.PrivatePort),
			Protocol:      p.Type,
		})
	}

	shortID := c.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	return model.Container{
		ID:                   c.ID,
		ShortID:              shortID,
		Name:                 name,
		Image:                c.Image,
		ImageID:              c.ImageID,
		Status:               c.Status,
		State:                c.State,
		CreatedAt:            time.Unix(c.Created, 0).UTC(),
		Ports:                ports,
		ComposeManagedFilter: isCompose,
	}
}
