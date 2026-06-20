package handlers

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"api-mocker/config"
	"api-mocker/probe"
	"api-mocker/websocket"
)

type Handler struct {
	db        *sqlx.DB
	rdb       *redis.Client
	cfg       *config.Config
	scheduler *probe.Scheduler
	wsHub     *websocket.Hub
}

func New(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, scheduler *probe.Scheduler, wsHub *websocket.Hub) *Handler {
	return &Handler{db: db, rdb: rdb, cfg: cfg, scheduler: scheduler, wsHub: wsHub}
}
