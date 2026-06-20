package handlers

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"api-mocker/config"
)

type Handler struct {
	db  *sqlx.DB
	rdb *redis.Client
	cfg *config.Config
}

func New(db *sqlx.DB, rdb *redis.Client, cfg *config.Config) *Handler {
	return &Handler{db: db, rdb: rdb, cfg: cfg}
}
