package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"api-mocker/config"
)

var createTablesSQL = `
CREATE TABLE IF NOT EXISTS api_dependencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    upstream_api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    downstream_api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    field_mappings JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(upstream_api_id, downstream_api_id)
);

CREATE TABLE IF NOT EXISTS impact_reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    changed_api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    changed_api_path VARCHAR(500) NOT NULL,
    changed_api_method VARCHAR(10) NOT NULL,
    change_type VARCHAR(20) NOT NULL,
    changed_fields JSONB NOT NULL DEFAULT '[]',
    affected_downstream JSONB NOT NULL DEFAULT '[]',
    has_breaking_change BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_dependencies_project ON api_dependencies(project_id);
CREATE INDEX IF NOT EXISTS idx_api_dependencies_upstream ON api_dependencies(upstream_api_id);
CREATE INDEX IF NOT EXISTS idx_api_dependencies_downstream ON api_dependencies(downstream_api_id);
CREATE INDEX IF NOT EXISTS idx_impact_reports_project ON impact_reports(project_id);
CREATE INDEX IF NOT EXISTS idx_impact_reports_created ON impact_reports(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_impact_reports_changed_api ON impact_reports(changed_api_id);
`

func Connect(cfg *config.Config) *sqlx.DB {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	fmt.Println("Connected to PostgreSQL")

	if _, err := db.Exec(createTablesSQL); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
	fmt.Println("Ensured api_dependencies and impact_reports tables exist")

	return db
}
