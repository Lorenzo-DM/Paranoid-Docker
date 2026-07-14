package service

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/model"
	"backend/internal/repository"
)

type SaveProgress struct {
	Type         string `json:"type"`
	WrittenBytes int64  `json:"written_bytes,omitempty"`
	Message      string `json:"message,omitempty"`
	Filename     string `json:"filename,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	Error        string `json:"error,omitempty"`
}

type ImageSaverService interface {
	SaveImage(ctx context.Context, containerID string, progressCh chan<- SaveProgress) error
	ListSavedImages() ([]model.SavedImage, error)
}

type imageSaverService struct {
	repo    repository.ContainerRepository
	baseDir string
}

func NewImageSaverService(repo repository.ContainerRepository, baseDir string) ImageSaverService {
	return &imageSaverService{repo: repo, baseDir: baseDir}
}

func (s *imageSaverService) SaveImage(ctx context.Context, containerID string, progressCh chan<- SaveProgress) error {
	defer close(progressCh)

	inspect, err := s.repo.InspectContainer(ctx, containerID)
	if err != nil {
		progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("inspect container: %s", err)}
		return err
	}

	imageID := inspect.Image
	imageRef := ""
	if inspect.Config != nil {
		imageRef = inspect.Config.Image
	}

	shortDigest := imageID
	if strings.HasPrefix(shortDigest, "sha256:") {
		shortDigest = shortDigest[7:15]
	}
	safeName := strings.ReplaceAll(imageRef, "/", "_")
	safeName = strings.ReplaceAll(safeName, ":", "_")
	filename := fmt.Sprintf("%s_%s.tar.gz", safeName, shortDigest)

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("create images dir: %s", err)}
		return err
	}

	path := filepath.Join(s.baseDir, filename)
	f, err := os.Create(path)
	if err != nil {
		progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("create file: %s", err)}
		return err
	}
	defer f.Close()

	rc, err := s.repo.SaveImage(ctx, imageID)
	if err != nil {
		progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("docker save: %s", err)}
		return err
	}
	defer rc.Close()

	progressCh <- SaveProgress{Type: "progress", Message: "Saving image (compressing)...", WrittenBytes: 0}

	gw := gzip.NewWriter(f)
	defer gw.Close()

	buf := make([]byte, 512*1024)
	var total int64
	for {
		n, readErr := rc.Read(buf)
		if n > 0 {
			if _, writeErr := gw.Write(buf[:n]); writeErr != nil {
				progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("write: %s", writeErr)}
				return writeErr
			}
			total += int64(n)
			progressCh <- SaveProgress{Type: "progress", WrittenBytes: total, Message: "Saving image (compressing)..."}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			progressCh <- SaveProgress{Type: "error", Error: fmt.Sprintf("read: %s", readErr)}
			return readErr
		}
	}

	progressCh <- SaveProgress{
		Type:      "done",
		Filename:  filename,
		SizeBytes: total,
	}
	return nil
}

func (s *imageSaverService) ListSavedImages() ([]model.SavedImage, error) {
	entries, err := os.ReadDir(s.baseDir)
	if os.IsNotExist(err) {
		return []model.SavedImage{}, nil
	}
	if err != nil {
		return nil, err
	}

	var images []model.SavedImage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		images = append(images, model.SavedImage{
			Filename:  e.Name(),
			Path:      filepath.Join(s.baseDir, e.Name()),
			ImageRef:  filenameToImageRef(e.Name()),
			SizeBytes: info.Size(),
			SavedAt:   info.ModTime(),
		})
	}
	return images, nil
}

func filenameToImageRef(filename string) string {
	name := strings.TrimSuffix(filename, ".tar.gz")

	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "_")
	}
	return name
}

func (sp SaveProgress) ToSSEEvent() string {
	event := sp.Type
	if event == "" {
		event = "progress"
	}
	return fmt.Sprintf("event: %s\ndata: {\"written_bytes\":%d,\"message\":%q,\"filename\":%q,\"size_bytes\":%d,\"error\":%q}\n\n",
		event, sp.WrittenBytes, sp.Message, sp.Filename, sp.SizeBytes, sp.Error)
}

func saveTimeFromFilename(_ string) time.Time {
	return time.Now()
}
