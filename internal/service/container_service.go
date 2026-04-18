package service

import (
	"backend/internal/model"
)

type ContainerService interface {
	GetAll() ([]model.Container, error)
}

type containerService struct {
}

func NewContainerService() ContainerService {
	return &containerService{}
}

func (s *containerService) GetAll() ([]model.Container, error) {
	return []model.Container{}, nil
}
