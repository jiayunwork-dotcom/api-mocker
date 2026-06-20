package handlers

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"api-mocker/config"
	"api-mocker/probe"
)

type Handler struct {
	db        *sqlx.DB
	rdb       *redis.Client
	cfg       *config.Config
	scheduler *probe.Scheduler
}

func New(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, scheduler *probe.Scheduler) *Handler {
	return &Handler{db: db, rdb: rdb, cfg: cfg, scheduler: scheduler}
}
