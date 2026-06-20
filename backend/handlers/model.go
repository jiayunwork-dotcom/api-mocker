package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

func (h *Handler) ListModels(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var models []models.SharedModel
	err := h.db.Select(&models,
		"SELECT * FROM shared_models WHERE project_id = $1 ORDER BY name ASC",
		projectID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list models"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *Handler) CreateModel(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		Name             string          `json:"name" binding:"required"`
		Description      string          `json:"description"`
		SchemaDefinition json.RawMessage `json:"schemaDefinition" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.checkCircularRef(projectID, req.Name, req.SchemaDefinition); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	model := models.SharedModel{
		ID:               uuid.New().String(),
		ProjectID:        projectID,
		Name:             req.Name,
		Description:      req.Description,
		SchemaDefinition: models.JSONB(req.SchemaDefinition),
	}

	_, err = h.db.Exec(
		"INSERT INTO shared_models (id, project_id, name, description, schema_definition) VALUES ($1, $2, $3, $4, $5)",
		model.ID, model.ProjectID, model.Name, model.Description, model.SchemaDefinition,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create model"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"model": model})
}

func (h *Handler) GetModel(c *gin.Context) {
	projectID := c.Param("projectId")
	modelID := c.Param("id")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var model models.SharedModel
	err := h.db.Get(&model, "SELECT * FROM shared_models WHERE id = $1 AND project_id = $2", modelID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"model": model})
}

func (h *Handler) UpdateModel(c *gin.Context) {
	projectID := c.Param("projectId")
	modelID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		Name             string          `json:"name"`
		Description      string          `json:"description"`
		SchemaDefinition json.RawMessage `json:"schemaDefinition"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var old models.SharedModel
	h.db.Get(&old, "SELECT * FROM shared_models WHERE id = $1 AND project_id = $2", modelID, projectID)

	name := req.Name
	if name == "" {
		name = old.Name
	}
	schema := req.SchemaDefinition
	if len(schema) == 0 {
		schema = json.RawMessage(old.SchemaDefinition)
	}

	if err := h.checkCircularRef(projectID, name, schema); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.db.Exec(
		"UPDATE shared_models SET name = $1, description = $2, schema_definition = $3, updated_at = NOW() WHERE id = $4",
		name, req.Description, schema, modelID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update model"})
		return
	}

	var updated models.SharedModel
	h.db.Get(&updated, "SELECT * FROM shared_models WHERE id = $1", modelID)
	c.JSON(http.StatusOK, gin.H{"model": updated})
}

func (h *Handler) DeleteModel(c *gin.Context) {
	projectID := c.Param("projectId")
	modelID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	_, err = h.db.Exec("DELETE FROM shared_models WHERE id = $1 AND project_id = $2", modelID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete model"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Model deleted"})
}

func (h *Handler) checkCircularRef(projectID, modelName string, schema json.RawMessage) error {
	var fields []models.BodyField
	if err := json.Unmarshal(schema, &fields); err != nil {
		return nil
	}

	refs := h.extractRefs(fields)
	if len(refs) == 0 {
		return nil
	}

	var allModels []models.SharedModel
	h.db.Select(&allModels, "SELECT name, schema_definition FROM shared_models WHERE project_id = $1", projectID)

	modelMap := map[string][]models.BodyField{}
	for _, m := range allModels {
		var f []models.BodyField
		json.Unmarshal([]byte(m.SchemaDefinition), &f)
		modelMap[m.Name] = f
	}

	var newFields []models.BodyField
	json.Unmarshal(schema, &newFields)
	modelMap[modelName] = newFields

	for _, ref := range refs {
		visited := map[string]bool{modelName: true}
		if h.hasCycle(ref, modelMap, visited) {
			return fmt.Errorf("Circular reference detected: model '%s' and '%s' form a cycle", modelName, ref)
		}
	}

	return nil
}

func (h *Handler) extractRefs(fields []models.BodyField) []string {
	var refs []string
	for _, f := range fields {
		if f.Ref != "" {
			refs = append(refs, f.Ref)
		}
		if len(f.Children) > 0 {
			refs = append(refs, h.extractRefs(f.Children)...)
		}
	}
	return refs
}

func (h *Handler) hasCycle(current string, modelMap map[string][]models.BodyField, visited map[string]bool) bool {
	if visited[current] {
		return true
	}
	visited[current] = true

	fields, exists := modelMap[current]
	if !exists {
		return false
	}

	for _, f := range fields {
		if f.Ref != "" {
			if h.hasCycle(f.Ref, modelMap, visited) {
				return true
			}
		}
		if len(f.Children) > 0 {
			refs := h.extractRefs(f.Children)
			for _, ref := range refs {
				if h.hasCycle(ref, modelMap, visited) {
					return true
				}
			}
		}
	}

	delete(visited, current)
	return false
}
