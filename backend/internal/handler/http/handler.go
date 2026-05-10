package http

import (
	"lumipluse-backend/internal/repository"
)

type Handler struct {
	Repo    repository.Repository
	Version string
}
