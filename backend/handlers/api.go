package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

var doubleSlashRegex = regexp.MustCompile(`//+`)

func (h *Handler) ListAPIs(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var apis []models.API
	err := h.db.Select(&apis,
		"SELECT * FROM apis WHERE project_id = $1 ORDER BY path ASC, method ASC",
		projectID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list APIs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"apis": apis})
}

func (h *Handler) CreateAPI(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var apiCount int
	h.db.Get(&apiCount, "SELECT COUNT(*) FROM apis WHERE project_id = $1", projectID)
	if apiCount >= 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project API limit (500) reached"})
		return
	}

	var req struct {
		Path        string          `json:"path" binding:"required"`
		Method      string          `json:"method" binding:"required"`
		Description string          `json:"description"`
		Params      json.RawMessage `json:"params"`
		RequestBody json.RawMessage `json:"requestBody"`
		Responses   json.RawMessage `json:"responses"`
		Tags        []string        `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !isValidPath(req.Path) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path must start with / and not contain consecutive double slashes"})
		return
	}

	if !isValidMethod(req.Method) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid HTTP method"})
		return
	}

	var existing int
	h.db.Get(&existing,
		"SELECT COUNT(*) FROM apis WHERE project_id = $1 AND path = $2 AND method = $3",
		projectID, req.Path, req.Method,
	)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "API with same path and method already exists in this project"})
		return
	}

	paramsJSON := models.JSONB(defaultJSON(req.Params, []byte("[]")))
	bodyJSON := models.JSONB(defaultJSON(req.RequestBody, []byte("{}")))
	responsesJSON := models.JSONB(defaultJSON(req.Responses, []byte("{}")))
	tags := models.StringArray(req.Tags)
	if tags == nil {
		tags = models.StringArray{}
	}

	api := models.API{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Path:        req.Path,
		Method:      strings.ToUpper(req.Method),
		Description: req.Description,
		Params:      paramsJSON,
		RequestBody: bodyJSON,
		Responses:   responsesJSON,
		Tags:        tags,
	}

	_, err = h.db.Exec(
		`INSERT INTO apis (id, project_id, path, method, description, params, request_body, responses, tags)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		api.ID, api.ProjectID, api.Path, api.Method, api.Description,
		api.Params, api.RequestBody, api.Responses, api.Tags,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API"})
		return
	}

	h.createVersion(api.ID, c.GetString("userID"), nil, api)

	c.JSON(http.StatusCreated, gin.H{"api": api})
}

func (h *Handler) GetAPI(c *gin.Context) {
	projectID := c.Param("projectId")
	apiID := c.Param("id")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var api models.API
	err := h.db.Get(&api, "SELECT * FROM apis WHERE id = $1 AND project_id = $2", apiID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"api": api})
}

func (h *Handler) UpdateAPI(c *gin.Context) {
	projectID := c.Param("projectId")
	apiID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var oldAPI models.API
	err = h.db.Get(&oldAPI, "SELECT * FROM apis WHERE id = $1 AND project_id = $2", apiID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API not found"})
		return
	}

	var req struct {
		Path        string          `json:"path"`
		Method      string          `json:"method"`
		Description string          `json:"description"`
		Params      json.RawMessage `json:"params"`
		RequestBody json.RawMessage `json:"requestBody"`
		Responses   json.RawMessage `json:"responses"`
		Tags        []string        `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newPath := req.Path
	if newPath == "" {
		newPath = oldAPI.Path
	}
	newMethod := req.Method
	if newMethod == "" {
		newMethod = oldAPI.Method
	}

	if !isValidPath(newPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path must start with / and not contain consecutive double slashes"})
		return
	}

	newMethod = strings.ToUpper(newMethod)
	if !isValidMethod(newMethod) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid HTTP method"})
		return
	}

	if newPath != oldAPI.Path || newMethod != oldAPI.Method {
		var existing int
		h.db.Get(&existing,
			"SELECT COUNT(*) FROM apis WHERE project_id = $1 AND path = $2 AND method = $3 AND id != $4",
			projectID, newPath, newMethod, apiID,
		)
		if existing > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "API with same path and method already exists"})
			return
		}
	}

	paramsJSON := models.JSONB(defaultJSON(req.Params, []byte(oldAPI.Params)))
	bodyJSON := models.JSONB(defaultJSON(req.RequestBody, []byte(oldAPI.RequestBody)))
	responsesJSON := models.JSONB(defaultJSON(req.Responses, []byte(oldAPI.Responses)))
	tags := models.StringArray(req.Tags)
	if tags == nil {
		tags = oldAPI.Tags
	}

	newAPI := models.API{
		ID:          apiID,
		ProjectID:   projectID,
		Path:        newPath,
		Method:      newMethod,
		Description: req.Description,
		Params:      paramsJSON,
		RequestBody: bodyJSON,
		Responses:   responsesJSON,
		Tags:        tags,
	}
	if newAPI.Description == "" {
		newAPI.Description = oldAPI.Description
	}

	_, err = h.db.Exec(
		`UPDATE apis SET path = $1, method = $2, description = $3, params = $4,
		 request_body = $5, responses = $6, tags = $7, updated_at = NOW() WHERE id = $8`,
		newAPI.Path, newAPI.Method, newAPI.Description,
		newAPI.Params, newAPI.RequestBody, newAPI.Responses, newAPI.Tags, apiID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update API"})
		return
	}

	h.createVersion(apiID, c.GetString("userID"), &oldAPI, newAPI)

	userID := c.GetString("userID")
	report, err := h.analyzeImpact(apiID, projectID, userID, oldAPI, newAPI)
	if err != nil {
		log.Printf("[impact-analysis] Failed to analyze impact: %v", err)
	} else if report != nil {
		var affected []models.AffectedDownstream
		json.Unmarshal([]byte(report.AffectedDownstream), &affected)

		if report.HasBreakingChange {
			h.BroadcastDependencyBreak(projectID, models.DependencyBreakMessage{
				ChangedAPIPath: report.ChangedAPIPath,
				AffectedCount:  len(affected),
				ReportID:       report.ID,
				ProjectID:      projectID,
			})
		}
	}

	var updated models.API
	h.db.Get(&updated, "SELECT * FROM apis WHERE id = $1", apiID)
	c.JSON(http.StatusOK, gin.H{"api": updated})
}

func (h *Handler) DeleteAPI(c *gin.Context) {
	projectID := c.Param("projectId")
	apiID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	_, err = h.db.Exec("DELETE FROM apis WHERE id = $1 AND project_id = $2", apiID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete API"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API deleted"})
}

func (h *Handler) ListScenarios(c *gin.Context) {
	apiID := c.Param("id")

	var scenarios []models.MockScenario
	err := h.db.Select(&scenarios,
		"SELECT * FROM mock_scenarios WHERE api_id = $1 ORDER BY priority ASC",
		apiID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list scenarios"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"scenarios": scenarios})
}

func (h *Handler) CreateScenario(c *gin.Context) {
	apiID := c.Param("id")

	var req struct {
		Name        string          `json:"name" binding:"required"`
		Description string          `json:"description"`
		Priority    int             `json:"priority"`
		Conditions  json.RawMessage `json:"conditions"`
		Response    json.RawMessage `json:"response"`
		StatusCode  int             `json:"statusCode"`
		DelayMs     int             `json:"delayMs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DelayMs > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mock delay must be under 200ms"})
		return
	}

	scenario := models.MockScenario{
		ID:          uuid.New().String(),
		APIID:       apiID,
		Name:        req.Name,
		Description: req.Description,
		Priority:    req.Priority,
		Conditions:  models.JSONB(defaultJSON(req.Conditions, []byte("[]"))),
		Response:    models.JSONB(defaultJSON(req.Response, []byte("{}"))),
		StatusCode:  defaultInt(req.StatusCode, 200),
		DelayMs:     req.DelayMs,
	}

	_, err := h.db.Exec(
		`INSERT INTO mock_scenarios (id, api_id, name, description, priority, conditions, response, status_code, delay_ms)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		scenario.ID, scenario.APIID, scenario.Name, scenario.Description,
		scenario.Priority, scenario.Conditions, scenario.Response, scenario.StatusCode, scenario.DelayMs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create scenario"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"scenario": scenario})
}

func (h *Handler) UpdateScenario(c *gin.Context) {
	scenarioID := c.Param("scenarioId")

	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Priority    int             `json:"priority"`
		Conditions  json.RawMessage `json:"conditions"`
		Response    json.RawMessage `json:"response"`
		StatusCode  int             `json:"statusCode"`
		DelayMs     int             `json:"delayMs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DelayMs > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mock delay must be under 200ms"})
		return
	}

	var old models.MockScenario
	h.db.Get(&old, "SELECT * FROM mock_scenarios WHERE id = $1", scenarioID)

	name := req.Name
	if name == "" {
		name = old.Name
	}
	statusCode := req.StatusCode
	if statusCode == 0 {
		statusCode = old.StatusCode
	}

	_, err := h.db.Exec(
		`UPDATE mock_scenarios SET name = $1, description = $2, priority = $3, conditions = $4,
		 response = $5, status_code = $6, delay_ms = $7, updated_at = NOW() WHERE id = $8`,
		name, req.Description, req.Priority,
		models.JSONB(defaultJSON(req.Conditions, []byte(old.Conditions))),
		models.JSONB(defaultJSON(req.Response, []byte(old.Response))),
		statusCode, req.DelayMs, scenarioID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update scenario"})
		return
	}

	var updated models.MockScenario
	h.db.Get(&updated, "SELECT * FROM mock_scenarios WHERE id = $1", scenarioID)
	c.JSON(http.StatusOK, gin.H{"scenario": updated})
}

func (h *Handler) DeleteScenario(c *gin.Context) {
	scenarioID := c.Param("scenarioId")

	_, err := h.db.Exec("DELETE FROM mock_scenarios WHERE id = $1", scenarioID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete scenario"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scenario deleted"})
}

func isValidPath(path string) bool {
	if path == "" {
		return false
	}
	if !strings.HasPrefix(path, "/") {
		return false
	}
	if doubleSlashRegex.MatchString(path) {
		return false
	}
	return true
}

func isValidMethod(method string) bool {
	valid := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true,
		"DELETE": true, "HEAD": true, "OPTIONS": true,
	}
	return valid[strings.ToUpper(method)]
}

func (h *Handler) canAccessProject(c *gin.Context, projectID string) bool {
	var workspaceID string
	err := h.db.Get(&workspaceID, "SELECT workspace_id FROM projects WHERE id = $1", projectID)
	if err != nil {
		return false
	}

	userID := c.GetString("userID")
	_, err = h.getWorkspaceRole(workspaceID, userID)
	return err == nil
}

func (h *Handler) getProjectRole(c *gin.Context, projectID string) (string, error) {
	var workspaceID string
	err := h.db.Get(&workspaceID, "SELECT workspace_id FROM projects WHERE id = $1", projectID)
	if err != nil {
		return "", err
	}

	userID := c.GetString("userID")
	return h.getWorkspaceRole(workspaceID, userID)
}

func defaultJSON(raw json.RawMessage, fallback json.RawMessage) json.RawMessage {
	if len(raw) > 0 {
		return raw
	}
	return fallback
}

func defaultInt(val, fallback int) int {
	if val == 0 {
		return fallback
	}
	return val
}
