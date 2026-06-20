CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE workspace_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'editor', 'viewer')),
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, user_id)
);

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    base_path VARCHAR(500) DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE shared_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    schema_definition JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE TABLE apis (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    path VARCHAR(500) NOT NULL,
    method VARCHAR(10) NOT NULL CHECK (method IN ('GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')),
    description TEXT,
    params JSONB DEFAULT '[]',
    request_body JSONB DEFAULT '{}',
    responses JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, path, method)
);

CREATE TABLE mock_scenarios (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    priority INT NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL DEFAULT '[]',
    response JSONB NOT NULL DEFAULT '{}',
    status_code INT NOT NULL DEFAULT 200,
    delay_ms INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE api_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    version INT NOT NULL,
    snapshot JSONB NOT NULL,
    change_summary TEXT,
    is_breaking BOOLEAN DEFAULT FALSE,
    changed_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'editor', 'viewer')),
    token UUID NOT NULL DEFAULT uuid_generate_v4(),
    invited_by UUID NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_workspace_members_workspace ON workspace_members(workspace_id);
CREATE INDEX idx_workspace_members_user ON workspace_members(user_id);
CREATE INDEX idx_apis_project ON apis(project_id);
CREATE INDEX idx_apis_path_method ON apis(project_id, path, method);
CREATE INDEX idx_mock_scenarios_api ON mock_scenarios(api_id);
CREATE INDEX idx_mock_scenarios_priority ON mock_scenarios(api_id, priority);
CREATE INDEX idx_api_versions_api ON api_versions(api_id);
CREATE INDEX idx_api_versions_created ON api_versions(api_id, created_at DESC);
CREATE INDEX idx_shared_models_project ON shared_models(project_id);
CREATE INDEX idx_invitations_token ON invitations(token);
CREATE INDEX idx_invitations_workspace ON invitations(workspace_id);

CREATE TABLE probe_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    api_id UUID NOT NULL REFERENCES apis(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    group_name VARCHAR(100) NOT NULL DEFAULT '',
    interval_seconds INT NOT NULL DEFAULT 30 CHECK (interval_seconds >= 10 AND interval_seconds <= 300),
    timeout_ms INT NOT NULL DEFAULT 3000 CHECK (timeout_ms > 0),
    fail_threshold INT NOT NULL DEFAULT 3 CHECK (fail_threshold > 0),
    recover_threshold INT NOT NULL DEFAULT 2 CHECK (recover_threshold > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'healthy' CHECK (status IN ('healthy', 'degraded', 'unhealthy')),
    consecutive_failures INT NOT NULL DEFAULT 0,
    consecutive_successes INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(api_id)
);

CREATE TABLE probe_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    probe_id UUID NOT NULL REFERENCES probe_configs(id) ON DELETE CASCADE,
    status_code INT NOT NULL DEFAULT 0,
    response_time_ms INT NOT NULL DEFAULT 0,
    response_size INT NOT NULL DEFAULT 0,
    is_success BOOLEAN NOT NULL DEFAULT FALSE,
    checked_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE alert_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    probe_id UUID NOT NULL REFERENCES probe_configs(id) ON DELETE CASCADE,
    probe_name VARCHAR(200) NOT NULL,
    old_status VARCHAR(20) NOT NULL,
    new_status VARCHAR(20) NOT NULL,
    last_response_time_ms INT NOT NULL DEFAULT 0,
    last_status_code INT NOT NULL DEFAULT 0,
    triggered_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_probe_configs_project ON probe_configs(project_id);
CREATE INDEX idx_probe_configs_enabled ON probe_configs(project_id, enabled);
CREATE INDEX idx_probe_records_probe ON probe_records(probe_id);
CREATE INDEX idx_probe_records_checked ON probe_records(probe_id, checked_at DESC);
CREATE INDEX idx_alert_events_probe ON alert_events(probe_id);
CREATE INDEX idx_alert_events_triggered ON alert_events(probe_id, triggered_at DESC);
