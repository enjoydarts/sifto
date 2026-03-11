package service

import (
	"context"

	"github.com/enjoydarts/sifto/api/internal/model"
	"github.com/enjoydarts/sifto/api/internal/repository"
)

type ItemDetailService struct {
	repo *repository.ItemRepo
}

func NewItemDetailService(repo *repository.ItemRepo) *ItemDetailService {
	return &ItemDetailService{repo: repo}
}

func (s *ItemDetailService) Get(ctx context.Context, itemID, userID string) (*model.ItemDetail, error) {
	return s.repo.GetDetail(ctx, itemID, userID)
}

type DigestDetailService struct {
	repo *repository.DigestRepo
}

func NewDigestDetailService(repo *repository.DigestRepo) *DigestDetailService {
	return &DigestDetailService{repo: repo}
}

func (s *DigestDetailService) Get(ctx context.Context, digestID, userID string) (*model.DigestDetail, error) {
	return s.repo.GetDetail(ctx, digestID, userID)
}

func (s *DigestDetailService) GetLatest(ctx context.Context, userID string) (*model.DigestDetail, error) {
	return s.repo.GetLatest(ctx, userID)
}
