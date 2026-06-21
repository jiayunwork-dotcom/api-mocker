package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

func (h *Handler) ListDependencies(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var deps []models.APIDependency
	err := h.db.Select(&deps, `
		SELECT 
			d.*,
			ua.path AS upstream_path,
			ua.method AS upstream_method,
			da.path AS downstream_path,
			da.method AS downstream_method
		FROM api_dependencies d
		INNER JOIN apis ua ON d.upstream_api_id = ua.id
		INNER JOIN apis da ON d.downstream_api_id = da.id
		WHERE d.project_id = $1
		ORDER BY d.created_at DESC
	`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list dependencies"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dependencies": deps})
}

func (h *Handler) CreateDependency(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		UpstreamAPIID   string                   `json:"upstream_api_id" binding:"required"`
		DownstreamAPIID string                   `json:"downstream_api_id" binding:"required"`
		FieldMappings   []models.FieldMapping    `json:"field_mappings" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.UpstreamAPIID == req.DownstreamAPIID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Upstream and downstream API cannot be the same"})
		return
	}

	var existing int
	err = h.db.Get(&existing, `
		SELECT COUNT(*) FROM api_dependencies 
		WHERE upstream_api_id = $1 AND downstream_api_id = $2
	`, req.UpstreamAPIID, req.DownstreamAPIID)
	if err != nil {
		log.Printf("[CreateDependency] Failed to check existing dependency: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing dependency: " + err.Error()})
		return
	}
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Dependency between these APIs already exists"})
		return
	}

	mappingsJSON, _ := json.Marshal(req.FieldMappings)

	dep := models.APIDependency{
		ID:              uuid.New().String(),
		ProjectID:       projectID,
		UpstreamAPIID:   req.UpstreamAPIID,
		DownstreamAPIID: req.DownstreamAPIID,
		FieldMappings:   models.JSONB(mappingsJSON),
	}

	log.Printf("[CreateDependency] Inserting dep: id=%s, project=%s, upstream=%s, downstream=%s, mappings=%s",
		dep.ID, dep.ProjectID, dep.UpstreamAPIID, dep.DownstreamAPIID, string(dep.FieldMappings))

	_, err = h.db.Exec(`
		INSERT INTO api_dependencies (id, project_id, upstream_api_id, downstream_api_id, field_mappings)
		VALUES ($1, $2, $3, $4, $5)
	`, dep.ID, dep.ProjectID, dep.UpstreamAPIID, dep.DownstreamAPIID, []byte(dep.FieldMappings))
	if err != nil {
		log.Printf("[CreateDependency] Failed to insert dependency: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create dependency: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"dependency": dep})
}

func (h *Handler) UpdateDependency(c *gin.Context) {
	projectID := c.Param("projectId")
	depID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var dep models.APIDependency
	err = h.db.Get(&dep, `
		SELECT * FROM api_dependencies WHERE id = $1 AND project_id = $2
	`, depID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dependency not found"})
		return
	}

	var req struct {
		FieldMappings []models.FieldMapping `json:"field_mappings" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mappingsJSON, _ := json.Marshal(req.FieldMappings)

	_, err = h.db.Exec(`
		UPDATE api_dependencies SET field_mappings = $1, updated_at = NOW() WHERE id = $2
	`, []byte(mappingsJSON), depID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update dependency"})
		return
	}

	var updated models.APIDependency
	h.db.Get(&updated, `
		SELECT 
			d.*,
			ua.path AS upstream_path,
			ua.method AS upstream_method,
			da.path AS downstream_path,
			da.method AS downstream_method
		FROM api_dependencies d
		INNER JOIN apis ua ON d.upstream_api_id = ua.id
		INNER JOIN apis da ON d.downstream_api_id = da.id
		WHERE d.id = $1
	`, depID)

	c.JSON(http.StatusOK, gin.H{"dependency": updated})
}

func (h *Handler) DeleteDependency(c *gin.Context) {
	projectID := c.Param("projectId")
	depID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	_, err = h.db.Exec(`
		DELETE FROM api_dependencies WHERE id = $1 AND project_id = $2
	`, depID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete dependency"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Dependency deleted"})
}

func (h *Handler) BatchCreateDependencies(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var items []struct {
		Upstream   string `json:"upstream" binding:"required"`
		Downstream string `json:"downstream" binding:"required"`
		Mappings   []struct {
			From string `json:"from" binding:"required"`
			To   string `json:"to" binding:"required"`
		} `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type apiLookup struct {
		ID     string
		Method string
		Path   string
	}
	var projectAPIs []apiLookup
	err = h.db.Select(&projectAPIs, `SELECT id, method, path FROM apis WHERE project_id = $1`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load project APIs"})
		return
	}
	apiMap := make(map[string]apiLookup)
	for _, a := range projectAPIs {
		key := a.Method + " " + a.Path
		apiMap[key] = a
	}

	created := 0
	skipped := 0

	for _, item := range items {
		upstream, okUp := apiMap[item.Upstream]
		downstream, okDown := apiMap[item.Downstream]

		if !okUp || !okDown {
			skipped++
			continue
		}

		if upstream.ID == downstream.ID {
			skipped++
			continue
		}

		var existing int
		err = h.db.Get(&existing, `
			SELECT COUNT(*) FROM api_dependencies
			WHERE upstream_api_id = $1 AND downstream_api_id = $2
		`, upstream.ID, downstream.ID)
		if err != nil || existing > 0 {
			skipped++
			continue
		}

		var mappings []models.FieldMapping
		for _, m := range item.Mappings {
			mappings = append(mappings, models.FieldMapping{
				UpstreamField:   m.From,
				DownstreamField: m.To,
			})
		}
		if len(mappings) == 0 {
			mappings = []models.FieldMapping{{UpstreamField: "", DownstreamField: ""}}
		}

		mappingsJSON, _ := json.Marshal(mappings)
		depID := uuid.New().String()

		_, err = h.db.Exec(`
			INSERT INTO api_dependencies (id, project_id, upstream_api_id, downstream_api_id, field_mappings)
			VALUES ($1, $2, $3, $4, $5)
		`, depID, projectID, upstream.ID, downstream.ID, []byte(mappingsJSON))
		if err != nil {
			log.Printf("[BatchCreateDependencies] Failed to insert dep: %v", err)
			skipped++
			continue
		}

		created++
	}

	c.JSON(http.StatusOK, gin.H{"created": created, "skipped": skipped})
}

func (h *Handler) GetImpactChain(c *gin.Context) {
	projectID := c.Param("projectId")
	reportID := c.Param("id")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var report models.ImpactReport
	err := h.db.Get(&report, `
		SELECT * FROM impact_reports WHERE id = $1 AND project_id = $2
	`, reportID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	type chainNode struct {
		APIID    string               `json:"api_id"`
		Method   string               `json:"method"`
		Path     string               `json:"path"`
		Level    int                  `json:"level"`
		Impact   string               `json:"impact"`
		Mappings []string             `json:"mappings"`
		Children []chainNode          `json:"children"`
	}

	var allDeps []models.APIDependency
	err = h.db.Select(&allDeps, `
		SELECT
			d.*,
			ua.path AS upstream_path,
			ua.method AS upstream_method,
			da.path AS downstream_path,
			da.method AS downstream_method
		FROM api_dependencies d
		INNER JOIN apis ua ON d.upstream_api_id = ua.id
		INNER JOIN apis da ON d.downstream_api_id = da.id
		WHERE d.project_id = $1
	`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load dependencies"})
		return
	}

	depMap := make(map[string][]models.APIDependency)
	for _, d := range allDeps {
		depMap[d.UpstreamAPIID] = append(depMap[d.UpstreamAPIID], d)
	}

	var affectedDownstream []models.AffectedDownstream
	json.Unmarshal([]byte(report.AffectedDownstream), &affectedDownstream)

	affectedFields := make(map[string][]string)
	affectedLevel := make(map[string]string)
	for _, ad := range affectedDownstream {
		affectedFields[ad.DownstreamAPIID] = ad.AffectedMappings
		affectedLevel[ad.DownstreamAPIID] = ad.ImpactLevel
	}

	maxDepth := 5
	visited := make(map[string]bool)

	var buildChain func(apiID string, level int) []chainNode
	buildChain = func(apiID string, level int) []chainNode {
		if level > maxDepth {
			return nil
		}
		if visited[apiID] {
			return nil
		}
		visited[apiID] = true

		deps := depMap[apiID]
		var children []chainNode
		for _, dep := range deps {
			impact := ""
			mappings := []string{}

			var fm []models.FieldMapping
			json.Unmarshal([]byte(dep.FieldMappings), &fm)

			if level == 1 {
				if fields, ok := affectedFields[dep.DownstreamAPIID]; ok {
					impact = affectedLevel[dep.DownstreamAPIID]
					mappings = fields
				}
			}

			if impact == "" {
				for _, m := range fm {
					for _, af := range affectedFields[apiID] {
						if m.UpstreamField == af || strings.HasPrefix(af, m.UpstreamField) {
							impact = "indirect"
							mappings = append(mappings, m.UpstreamField+" -> "+m.DownstreamField)
						}
					}
				}
			}

			node := chainNode{
				APIID:    dep.DownstreamAPIID,
				Method:   dep.DownstreamMethod,
				Path:     dep.DownstreamPath,
				Level:    level,
				Impact:   impact,
				Mappings: mappings,
			}

			subChildren := buildChain(dep.DownstreamAPIID, level+1)
			if subChildren != nil {
				node.Children = subChildren
			}

			children = append(children, node)
		}

		visited[apiID] = false
		return children
	}

	chain := buildChain(report.ChangedAPIID, 1)

	c.JSON(http.StatusOK, gin.H{
		"chain":          chain,
		"changed_api_id": report.ChangedAPIID,
		"changed_path":   report.ChangedAPIPath,
		"changed_method": report.ChangedAPIMethod,
	})
}

func (h *Handler) ListImpactReports(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var reports []models.ImpactReport
	err := h.db.Select(&reports, `
		SELECT 
			r.*,
			u.name AS user_name
		FROM impact_reports r
		INNER JOIN users u ON r.created_by = u.id
		WHERE r.project_id = $1
		ORDER BY r.created_at DESC
		LIMIT 50
	`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list impact reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reports": reports})
}

func (h *Handler) GetImpactReport(c *gin.Context) {
	projectID := c.Param("projectId")
	reportID := c.Param("id")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var report models.ImpactReport
	err := h.db.Get(&report, `
		SELECT 
			r.*,
			u.name AS user_name
		FROM impact_reports r
		INNER JOIN users u ON r.created_by = u.id
		WHERE r.id = $1 AND r.project_id = $2
	`, reportID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"report": report})
}
