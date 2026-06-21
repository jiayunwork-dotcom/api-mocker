package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

func (h *Handler) ListProjects(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	userID := c.GetString("userID")

	_, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}

	var projects []models.Project
	err = h.db.Select(&projects,
		"SELECT * FROM projects WHERE workspace_id = $1 ORDER BY updated_at DESC",
		workspaceID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

func (h *Handler) CreateProject(c *gin.Context) {
	workspaceID := c.Param("workspaceId")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		BasePath    string `json:"basePath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project := models.Project{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		BasePath:    req.BasePath,
	}

	_, err = h.db.Exec(
		"INSERT INTO projects (id, workspace_id, name, description, base_path) VALUES ($1, $2, $3, $4, $5)",
		project.ID, project.WorkspaceID, project.Name, project.Description, project.BasePath,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func (h *Handler) GetProject(c *gin.Context) {
	projectID := c.Param("projectId")
	workspaceID := c.Param("workspaceId")
	userID := c.GetString("userID")

	_, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}

	var project models.Project
	err = h.db.Get(&project, "SELECT * FROM projects WHERE id = $1 AND workspace_id = $2", projectID, workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *Handler) GetProjectByID(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var project models.Project
	err := h.db.Get(&project, "SELECT * FROM projects WHERE id = $1", projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *Handler) UpdateProject(c *gin.Context) {
	projectID := c.Param("projectId")
	workspaceID := c.Param("workspaceId")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		BasePath    string `json:"basePath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.db.Exec(
		"UPDATE projects SET name = $1, description = $2, base_path = $3, updated_at = NOW() WHERE id = $4 AND workspace_id = $5",
		req.Name, req.Description, req.BasePath, projectID, workspaceID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project"})
		return
	}

	var project models.Project
	h.db.Get(&project, "SELECT * FROM projects WHERE id = $1", projectID)
	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (h *Handler) DeleteProject(c *gin.Context) {
	projectID := c.Param("projectId")
	workspaceID := c.Param("workspaceId")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}
	if role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	_, err = h.db.Exec("DELETE FROM projects WHERE id = $1 AND workspace_id = $2", projectID, workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete project"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Project deleted"})
}
