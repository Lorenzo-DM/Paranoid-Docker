package service

import (
	"context"
	"strings"

	"backend/internal/repository"
)

type DigestChecker interface {
	GetLocalTagDigest(ctx context.Context, imageRef string) (string, error)
}

type digestChecker struct {
	repo repository.ContainerRepository
}

func NewDigestChecker(repo repository.ContainerRepository) DigestChecker {
	return &digestChecker{repo: repo}
}

func (d *digestChecker) GetLocalTagDigest(ctx context.Context, imageRef string) (string, error) {
	inspect, err := d.repo.InspectImage(ctx, imageRef)
	if err != nil {
		return "", err
	}

	for _, rd := range inspect.RepoDigests {
		if idx := strings.Index(rd, "@"); idx != -1 {
			return rd[idx+1:], nil
		}
	}

	return "", nil
}
