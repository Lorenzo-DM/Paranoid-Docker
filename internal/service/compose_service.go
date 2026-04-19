package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"backend/internal/model"
	"backend/internal/repository"

	dockertypes "github.com/docker/docker/api/types"
)

type ComposeStackService interface {
	ListStacks(ctx context.Context) ([]model.ComposeStack, error)
	UpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error
	SaveStackImages(ctx context.Context, name string, eventCh chan<- model.StackEvent) error
	SaveAndUpdateStack(ctx context.Context, name string, eventCh chan<- model.StackEvent, includeEnv bool) error
	SnapshotStack(ctx context.Context, name string, includeEnv bool) (string, error)
	StreamStackLogs(ctx context.Context, name string, w io.Writer) error
	ListRollbacksForStack(name string) ([]model.RollbackFile, error)
}

type composeStackService struct {
	repo          repository.ContainerRepository
	digestChecker DigestChecker
	imageSaver    ImageSaverService
}

func NewComposeStackService(repo repository.ContainerRepository, dc DigestChecker, is ImageSaverService) ComposeStackService {
	return &composeStackService{repo: repo, digestChecker: dc, imageSaver: is}
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
	return WriteStackRollback(ctx, *stack, s.repo, includeEnv)
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
	if len(stack.ConfigFiles) == 0 {
		err := fmt.Errorf("no compose config file found for stack %q", stack.Name)
		eventCh <- model.StackEvent{Type: "error", Error: err.Error()}
		return err
	}

	eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: "Saving rollback snapshot..."}
	rollbackDir, err := WriteStackRollback(ctx, *stack, s.repo, includeEnv)
	if err != nil {
		eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: fmt.Sprintf("WARNING: rollback snapshot failed: %s", err)}
	} else {
		eventCh <- model.StackEvent{Type: "progress", Step: "rollback", Line: fmt.Sprintf("Rollback saved to %s", rollbackDir)}
	}

	configArgs := composeFileArgs(stack.ConfigFiles)

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
	cmd := exec.CommandContext(ctx, "docker", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		eventCh <- model.StackEvent{Type: "error", Step: step, Error: err.Error()}
		return err
	}

	done := make(chan struct{}, 2)
	stream := func(r io.Reader) {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if line := sc.Text(); line != "" {
				eventCh <- model.StackEvent{Type: "progress", Step: step, Line: line}
			}
		}
		done <- struct{}{}
	}
	go stream(stdout)
	go stream(stderr)
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		eventCh <- model.StackEvent{Type: "error", Step: step, Error: fmt.Sprintf("docker compose %s: %s", step, err)}
		return err
	}
	return nil
}

func (s *composeStackService) StreamStackLogs(ctx context.Context, name string, w io.Writer) error {
	stacks, err := s.ListStacks(ctx)
	if err != nil {
		return err
	}
	for _, st := range stacks {
		if st.Name == name {
			if len(st.ConfigFiles) == 0 {
				return fmt.Errorf("no config files for stack %q", name)
			}
			args := append([]string{"compose"}, composeFileArgs(st.ConfigFiles)...)
			args = append(args, "logs", "--follow", "--no-color", "--timestamps")
			cmd := exec.CommandContext(ctx, "docker", args...)
			if st.WorkingDir != "" {
				cmd.Dir = st.WorkingDir
			}
			cmd.Stdout = w
			cmd.Stderr = w
			return cmd.Run()
		}
	}
	return fmt.Errorf("stack %q not found", name)
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

func composeFileArgs(files []string) []string {
	args := make([]string, 0, len(files)*2)
	for _, f := range files {
		args = append(args, "-f", f)
	}
	return args
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
