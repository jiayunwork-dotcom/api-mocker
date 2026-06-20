package models

import "time"

type User struct {
	ID           string    `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	Name         string    `db:"name" json:"name"`
	PasswordHash string    `db:"password_hash" json:"-"`
	AvatarURL    string    `db:"avatar_url" json:"avatar_url,omitempty"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type Workspace struct {
	ID          string    `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description,omitempty"`
	OwnerID     string    `db:"owner_id" json:"owner_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type WorkspaceMember struct {
	ID          string    `db:"id" json:"id"`
	WorkspaceID string    `db:"workspace_id" json:"workspace_id"`
	UserID      string    `db:"user_id" json:"user_id"`
	Role        string    `db:"role" json:"role"`
	JoinedAt    time.Time `db:"joined_at" json:"joined_at"`
	UserName    string    `db:"user_name" json:"user_name,omitempty"`
	UserEmail   string    `db:"user_email" json:"user_email,omitempty"`
}

type Project struct {
	ID          string    `db:"id" json:"id"`
	WorkspaceID string    `db:"workspace_id" json:"workspace_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description,omitempty"`
	BasePath    string    `db:"base_path" json:"base_path,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type SharedModel struct {
	ID               string    `db:"id" json:"id"`
	ProjectID        string    `db:"project_id" json:"project_id"`
	Name             string    `db:"name" json:"name"`
	Description      string    `db:"description" json:"description,omitempty"`
	SchemaDefinition any       `db:"schema_definition" json:"schema_definition"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

type API struct {
	ID          string    `db:"id" json:"id"`
	ProjectID   string    `db:"project_id" json:"project_id"`
	Path        string    `db:"path" json:"path"`
	Method      string    `db:"method" json:"method"`
	Description string    `db:"description" json:"description,omitempty"`
	Params      any       `db:"params" json:"params"`
	RequestBody any       `db:"request_body" json:"request_body"`
	Responses   any       `db:"responses" json:"responses"`
	Tags        []string  `db:"tags" json:"tags"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type MockScenario struct {
	ID          string    `db:"id" json:"id"`
	APIID       string    `db:"api_id" json:"api_id"`
	Name        string    `db:"name" json:"name"`
	Description string    `db:"description" json:"description,omitempty"`
	Priority    int       `db:"priority" json:"priority"`
	Conditions  any       `db:"conditions" json:"conditions"`
	Response    any       `db:"response" json:"response"`
	StatusCode  int       `db:"status_code" json:"status_code"`
	DelayMs     int       `db:"delay_ms" json:"delay_ms"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

type APIVersion struct {
	ID            string    `db:"id" json:"id"`
	APIID         string    `db:"api_id" json:"api_id"`
	Version       int       `db:"version" json:"version"`
	Snapshot      any       `db:"snapshot" json:"snapshot"`
	ChangeSummary string    `db:"change_summary" json:"change_summary,omitempty"`
	IsBreaking    bool      `db:"is_breaking" json:"is_breaking"`
	ChangedBy     string    `db:"changed_by" json:"changed_by"`
	ChangerName   string    `db:"changer_name" json:"changer_name,omitempty"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type Invitation struct {
	ID          string     `db:"id" json:"id"`
	WorkspaceID string     `db:"workspace_id" json:"workspace_id"`
	Email       string     `db:"email" json:"email"`
	Role        string     `db:"role" json:"role"`
	Token       string     `db:"token" json:"token"`
	InvitedBy   string     `db:"invited_by" json:"invited_by"`
	ExpiresAt   time.Time  `db:"expires_at" json:"expires_at"`
	AcceptedAt  *time.Time `db:"accepted_at" json:"accepted_at,omitempty"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

type ParamField struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
	Example  string `json:"example,omitempty"`
	Desc     string `json:"desc,omitempty"`
}

type BodyField struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Example  interface{} `json:"example,omitempty"`
	Desc     string      `json:"desc,omitempty"`
	Children []BodyField `json:"children,omitempty"`
	Ref      string      `json:"ref,omitempty"`
	Enum     []string    `json:"enum,omitempty"`
}

type ResponseDef struct {
	Description string      `json:"description"`
	Body        []BodyField `json:"body"`
}

type ConditionRule struct {
	Field    string `json:"field"`
	In       string `json:"in"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}
