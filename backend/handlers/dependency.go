package handlers

import (
	"encoding/json"
	"net/http"

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing dependency"})
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

	_, err = h.db.Exec(`
		INSERT INTO api_dependencies (id, project_id, upstream_api_id, downstream_api_id, field_mappings)
		VALUES ($1, $2, $3, $4, $5)
	`, dep.ID, dep.ProjectID, dep.UpstreamAPIID, dep.DownstreamAPIID, dep.FieldMappings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create dependency"})
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
	`, models.JSONB(mappingsJSON), depID)
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
