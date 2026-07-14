package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"backend/internal/model"
	"backend/internal/repository"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type ComposeStackService interface {
	ListStacks(ctx context.Context) ([]model.ComposeStack, error)
	UpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error
	SaveStackImages(ctx context.Context, name string, eventCh chan<- model.StackEvent) error
	SaveAndUpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error
	SnapshotStack(ctx context.Context, name string, includeEnv bool) (string, error)
	StreamStackLogs(ctx context.Context, name string, w io.Writer) error
	ListRollbacksForStack(name string) ([]model.RollbackFile, error)
	GetCapabilities(ctx context.Context) model.Capabilities
	GetRollbackMode() string
	SetRollbackMode(mode string)
}

type composeStackService struct {
	repo          repository.ContainerRepository
	digestChecker DigestChecker
	imageSaver    ImageSaverService
	runner        CommandRunner
	rollbackMode  string // "auto" | "compose" | "inspect"
}

func NewComposeStackService(repo repository.ContainerRepository, dc DigestChecker, is ImageSaverService, runner CommandRunner) ComposeStackService {
	return &composeStackService{repo: repo, digestChecker: dc, imageSaver: is, runner: runner, rollbackMode: "auto"}
}

func (s *composeStackService) GetRollbackMode() string { return s.rollbackMode }

func (s *composeStackService) SetRollbackMode(mode string) {
	if mode == "auto" || mode == "compose" || mode == "inspect" {
		s.rollbackMode = mode
	}
}

func (s *composeStackService) GetCapabilities(ctx context.Context) model.Capabilities {
	stacks, _ := s.ListStacks(ctx)
	anyAccessible := false
	for _, st := range stacks {
		if configFileAccessible(st.ConfigFiles) {
			anyAccessible = true
			break
		}
	}
	return model.Capabilities{
		ComposeFilesAvailable: anyAccessible,
		RollbackMode:          s.rollbackMode,
	}
}

func (s *composeStackService) effectiveMode(configFiles []string) string {
	switch s.rollbackMode {
	case "compose":
		return "compose"
	case "inspect":
		return "inspect"
	default: // "auto"
		if configFileAccessible(configFiles) {
			return "compose"
		}
		return "inspect"
	}
}

func (s *composeStackService) ListStacks(ctx context.Context) ([]model.ComposeStack, error) {
	containers, err := s.repo.ListContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	type projectData struct {
		configFiles string
		workingDir  string
		containers  []dockertypes.Container
	}
	projects := make(map[string]*projectData)

	for _, c := range containers {
		project := c.Labels["com.docker.compose.project"]
		if project == "" {
			continue
		}
		if projects[project] == nil {
			projects[project] = &projectData{
				configFiles: c.Labels["com.docker.compose.project.config_files"],
				workingDir:  c.Labels["com.docker.compose.project.working_dir"],
			}
		}
		projects[project].containers = append(projects[project].containers, c)
	}

	stacks := make([]model.ComposeStack, 0, len(projects))
	for name, pd := range projects {
		stack := model.ComposeStack{
			Name:        name,
			WorkingDir:  pd.workingDir,
			ConfigFiles: splitConfigFiles(pd.configFiles),
		}

		runningCount := 0
		for _, c := range pd.containers {
			svc := s.buildService(ctx, c)
			stack.Services = append(stack.Services, svc)
			if svc.State == "running" {
				runningCount++
			}
			if svc.UpdateAvailable {
				stack.UpdateAvailable = true
			}
		}

		stack.Status = stackStatus(runningCount, len(pd.containers))
		stack.RollbackMode = s.effectiveMode(stack.ConfigFiles)
		stacks = append(stacks, stack)
	}

	return stacks, nil
}

func (s *composeStackService) buildService(ctx context.Context, c dockertypes.Container) model.ComposeService {
	svcName := c.Labels["com.docker.compose.service"]
	if svcName == "" {
		if len(c.Names) > 0 {
			svcName = strings.TrimPrefix(c.Names[0], "/")
		}
	}

	ports := make([]model.Port, 0, len(c.Ports))
	for _, p := range c.Ports {
		ports = append(ports, model.Port{
			HostPort:      fmt.Sprintf("%d", p.PublicPort),
			ContainerPort: fmt.Sprintf("%d", p.PrivatePort),
			Protocol:      p.Type,
		})
	}

	svc := model.ComposeService{
		Name:        svcName,
		Image:       c.Image,
		ContainerID: c.ID[:12],
		State:       c.State,
		Status:      c.Status,
		Ports:       ports,
	}

	if !strings.Contains(c.Image, "@sha256:") {
		imgInspect, err := s.repo.InspectImage(ctx, c.ImageID)
		if err == nil {
			svc.LocalDigest = extractDigest(imgInspect)
			svc.ImageLabels = extractOCILabels(imgInspect.Config.Labels)
		}
		if svc.LocalDigest != "" {
			remote, err := s.digestChecker.GetLocalTagDigest(ctx, c.Image)
			if err == nil {
				svc.RemoteDigest = remote
				svc.UpdateAvailable = svc.LocalDigest != remote
			}
		}
	}

	return svc
}

func (s *composeStackService) UpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error {
	defer close(eventCh)

	stack, err := s.findStack(ctx, name)
	if err != nil {
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}
	if err := s.doUpdateStack(ctx, stack, eventCh, includeEnv); err != nil {
		return err
	}
	eventCh <- model.StackEvent{Type: "done", Line: "Stack updated successfully"}
	return nil
}

func (s *composeStackService) SaveStackImages(ctx context.Context, name string, eventCh chan<- model.StackEvent) error {
	defer close(eventCh)

	stack, err := s.findStack(ctx, name)
	if err != nil {
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}
	if err := s.doSaveStackImages(ctx, stack, eventCh); err != nil {
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}
	eventCh <- model.StackEvent{Type: "done", Line: "All images saved successfully"}
	return nil
}

func (s *composeStackService) SaveAndUpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error {
	defer close(eventCh)

	stack, err := s.findStack(ctx, name)
	if err != nil {
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}
	if err := s.doSaveStackImages(ctx, stack, eventCh); err != nil {
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}
	if err := s.doUpdateStack(ctx, stack, eventCh, includeEnv); err != nil {
		return err
	}
	eventCh <- model.StackEvent{Type: "done", Line: "Images saved and stack updated successfully"}
	return nil
}

func (s *composeStackService) SnapshotStack(ctx context.Context, name string, includeEnv bool) (string, error) {
	stack, err := s.findStack(ctx, name)
	if err != nil {
		return "", err
	}
	return WriteStackRollback(ctx, *stack, s.repo, includeEnv, s.effectiveMode(stack.ConfigFiles), s.runner)
}

func (s *composeStackService) findStack(ctx context.Context, name string) (*model.ComposeStack, error) {
	stacks, err := s.ListStacks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stacks: %w", err)
	}
	for i := range stacks {
		if stacks[i].Name == name {
			return &stacks[i], nil
		}
	}
	return nil, fmt.Errorf("stack %q not found", name)
}

func (s *composeStackService) doSaveStackImages(ctx context.Context, stack *model.ComposeStack, eventCh chan<- model.StackEvent) error {
	for _, svc := range stack.Services {
		eventCh <- model.StackEvent{Type: "progress", Step: "save", Line: fmt.Sprintf("Saving image for service %s...", svc.Name)}

		saveCh := make(chan SaveProgress, 64)
		fanDone := make(chan struct{})

		go func(serviceName string) {
			defer close(fanDone)
			for p := range saveCh {
				var line string
				switch {
				case p.Type == "done" && p.Filename != "":
					line = fmt.Sprintf("[%s] saved → images/%s (%.1f MB)", serviceName, p.Filename, float64(p.SizeBytes)/1024/1024)
				case p.WrittenBytes > 0:
					line = fmt.Sprintf("[%s] writing… %d MB", serviceName, p.WrittenBytes/1024/1024)
				}
				if line != "" {
					eventCh <- model.StackEvent{Type: "progress", Step: "save", Line: line}
				}
			}
		}(svc.Name)

		err := s.imageSaver.SaveImage(ctx, svc.ContainerID, saveCh)
		<-fanDone

		if err != nil {
			return fmt.Errorf("save image for %s: %w", svc.Name, err)
		}
	}
	return nil
}

func (s *composeStackService) doUpdateStack(ctx context.Context, stack *model.ComposeStack, eventCh chan<- model.StackEvent, includeEnv bool) error {
	eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: "Saving rollback snapshot..."}
	rollbackDir, err := WriteStackRollback(ctx, *stack, s.repo, includeEnv, s.effectiveMode(stack.ConfigFiles), s.runner)
	if err != nil {
		eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: fmt.Sprintf("WARNING: rollback snapshot failed: %s", err)}
	} else {
		eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: fmt.Sprintf("Rollback saved to %s", rollbackDir)}
	}

	if s.effectiveMode(stack.ConfigFiles) == "compose" {
		return s.doComposeFileUpdate(ctx, stack, eventCh)
	}
	return s.doInspectBasedUpdate(ctx, stack, eventCh)
}

func (s *composeStackService) doComposeFileUpdate(ctx context.Context, stack *model.ComposeStack, eventCh chan<- model.StackEvent) error {
	configArgs := make([]string, 0, len(stack.ConfigFiles)*2)
	for _, f := range stack.ConfigFiles {
		configArgs = append(configArgs, "-f", f)
	}

	eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: "Pulling latest images..."}
	pullArgs := append([]string{"compose"}, configArgs...)
	pullArgs = append(pullArgs, "pull")
	if err := s.runCompose(ctx, stack.WorkingDir, pullArgs, "pull", eventCh); err != nil {
		return err
	}

	eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: "Recreating containers..."}
	upArgs := append([]string{"compose"}, configArgs...)
	upArgs = append(upArgs, "up", "-d", "--remove-orphans")
	if err := s.runCompose(ctx, stack.WorkingDir, upArgs, "up", eventCh); err != nil {
		return err
	}

	return nil
}

func (s *composeStackService) runCompose(
	ctx context.Context,
	workDir string,
	args []string,
	step string,
	eventCh chan<- model.StackEvent,
) error {
	emit := func(line string) {
		eventCh <- model.StackEvent{Type: "progress", Step: step, Line: line}
	}
	stdout := &lineWriter{emit: emit}
	stderr := &lineWriter{emit: emit}

	err := s.runner.Stream(ctx, workDir, stdout, stderr, "docker", args...)
	stdout.Flush()
	stderr.Flush()

	if err != nil {
		eventCh <- model.StackEvent{Type: "error", Step: step, Error: fmt.Sprintf("docker compose %s: %s", step, err)}
		return err
	}
	return nil
}

func (s *composeStackService) doInspectBasedUpdate(ctx context.Context, stack *model.ComposeStack, eventCh chan<- model.StackEvent) error {
	for _, svc := range stack.Services {
		if svc.ContainerID == "" {
			continue
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: fmt.Sprintf("[%s] Inspecting container...", svc.Name)}

		inspect, err := s.repo.InspectContainer(ctx, svc.ContainerID)
		if err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: fmt.Sprintf("[%s] ERROR inspecting: %s", svc.Name, err)}
			continue
		}

		cfg := captureContainerConfig(inspect)

		eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: fmt.Sprintf("[%s] Pulling %s...", svc.Name, cfg.Image)}

		pullStream, err := s.repo.PullImage(ctx, cfg.Image)
		if err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: fmt.Sprintf("[%s] ERROR pulling: %s", svc.Name, err)}
			continue
		}

		pullCh := make(chan PullEvent, 64)
		go ParsePullStream(pullStream, pullCh)
		for evt := range pullCh {
			if evt.Status != "" {
				eventCh <- model.StackEvent{Type: "progress", Step: "pull", Line: fmt.Sprintf("[%s] %s", svc.Name, evt.Status)}
			}
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] Stopping...", svc.Name)}
		timeout := 10
		if err := s.repo.StopContainer(ctx, svc.ContainerID, &timeout); err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] ERROR stopping: %s", svc.Name, err)}
			continue
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] Removing...", svc.Name)}
		if err := s.repo.RemoveContainer(ctx, svc.ContainerID); err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] ERROR removing: %s", svc.Name, err)}
			continue
		}

		containerCfg := &container.Config{
			Image:      cfg.Image,
			Cmd:        cfg.Cmd,
			Entrypoint: cfg.Entrypoint,
			Env:        cfg.Env,
			Labels:     cfg.Labels,
		}
		hostCfg := &container.HostConfig{
			Binds:         cfg.Binds,
			PortBindings:  cfg.PortBindings,
			NetworkMode:   cfg.NetworkMode,
			RestartPolicy: cfg.RestartPolicy,
			AutoRemove:    cfg.AutoRemove,
		}

		var netCfg *network.NetworkingConfig
		primaryNet := string(cfg.NetworkMode)
		if primaryNet != "" && !strings.HasPrefix(primaryNet, "container:") &&
			primaryNet != "host" && primaryNet != "none" && primaryNet != "bridge" {
			netCfg = &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					primaryNet: {},
				},
			}
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] Creating...", svc.Name)}
		newID, err := s.repo.CreateContainer(ctx, cfg.Name, containerCfg, hostCfg, netCfg)
		if err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] ERROR creating: %s", svc.Name, err)}
			continue
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] Starting...", svc.Name)}
		if err := s.repo.StartContainer(ctx, newID); err != nil {
			eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] ERROR starting: %s", svc.Name, err)}
			continue
		}

		for _, netName := range cfg.Networks {
			_ = s.repo.ConnectNetwork(ctx, netName, newID, nil)
		}

		eventCh <- model.StackEvent{Type: "progress", Step: "up", Line: fmt.Sprintf("[%s] Updated successfully", svc.Name)}
	}

	return nil
}

func (s *composeStackService) StreamStackLogs(ctx context.Context, name string, w io.Writer) error {
	stacks, err := s.ListStacks(ctx)
	if err != nil {
		return err
	}
	for _, st := range stacks {
		if st.Name != name {
			continue
		}

		if s.effectiveMode(st.ConfigFiles) == "compose" {
			return s.streamComposeFileLogs(ctx, st, w)
		}
		return s.streamInspectLogs(ctx, st, w)
	}
	return fmt.Errorf("stack %q not found", name)
}

func (s *composeStackService) streamComposeFileLogs(ctx context.Context, st model.ComposeStack, w io.Writer) error {
	args := []string{"compose"}
	for _, f := range st.ConfigFiles {
		args = append(args, "-f", f)
	}
	args = append(args, "logs", "--follow", "--no-color", "--timestamps")
	return s.runner.Stream(ctx, st.WorkingDir, w, w, "docker", args...)
}

func (s *composeStackService) streamInspectLogs(ctx context.Context, st model.ComposeStack, w io.Writer) error {
	readers := make([]io.ReadCloser, 0, len(st.Services))
	for _, svc := range st.Services {
		if svc.ContainerID == "" {
			continue
		}
		rc, err := s.repo.ContainerLogs(ctx, svc.ContainerID, true)
		if err != nil {
			continue
		}
		readers = append(readers, rc)
	}

	for _, rc := range readers {
		go func(r io.ReadCloser) {
			defer r.Close()
			sc := bufio.NewScanner(r)
			for sc.Scan() {
				w.Write(sc.Bytes())
				w.Write([]byte("\n"))
			}
		}(rc)
	}

	<-ctx.Done()
	return nil
}

func (s *composeStackService) ListRollbacksForStack(name string) ([]model.RollbackFile, error) {
	return ListStackRollbacks(name)
}

func splitConfigFiles(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

func stackStatus(running, total int) string {
	switch {
	case running == 0:
		return "stopped"
	case running == total:
		return "running"
	default:
		return "partial"
	}
}

func extractDigest(inspect dockertypes.ImageInspect) string {
	for _, rd := range inspect.RepoDigests {
		if idx := strings.Index(rd, "@"); idx != -1 {
			return rd[idx+1:]
		}
	}
	return ""
}

func extractOCILabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	result := make(map[string]string)
	for k, v := range labels {
		if strings.HasPrefix(k, "org.opencontainers.image.") {
			result[k] = v
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
