package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

func (h *Handler) ListWorkspaces(c *gin.Context) {
	userID := c.GetString("userID")

	var workspaces []models.Workspace
	err := h.db.Select(&workspaces, `
		SELECT w.* FROM workspaces w
		LEFT JOIN workspace_members wm ON w.id = wm.workspace_id
		WHERE w.owner_id = $1 OR wm.user_id = $1
		GROUP BY w.id
		ORDER BY w.updated_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workspaces"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

func (h *Handler) CreateWorkspace(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workspace := models.Workspace{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO workspaces (id, name, description, owner_id) VALUES ($1, $2, $3, $4)",
		workspace.ID, workspace.Name, workspace.Description, workspace.OwnerID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}

	_, err = tx.Exec(
		"INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'admin')",
		workspace.ID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add owner as member"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"workspace": workspace})
}

func (h *Handler) GetWorkspace(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found or no access"})
		return
	}

	var workspace models.Workspace
	err = h.db.Get(&workspace, "SELECT * FROM workspaces WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workspace not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workspace": workspace, "role": role})
}

func (h *Handler) UpdateWorkspace(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(id, userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.db.Exec(
		"UPDATE workspaces SET name = $1, description = $2, updated_at = NOW() WHERE id = $3",
		req.Name, req.Description, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workspace"})
		return
	}

	var workspace models.Workspace
	h.db.Get(&workspace, "SELECT * FROM workspaces WHERE id = $1", id)
	c.JSON(http.StatusOK, gin.H{"workspace": workspace})
}

func (h *Handler) DeleteWorkspace(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(id, userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	_, err = h.db.Exec("DELETE FROM workspaces WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workspace"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Workspace deleted"})
}

func (h *Handler) getWorkspaceRole(workspaceID, userID string) (string, error) {
	var role string
	err := h.db.Get(&role,
		"SELECT role FROM workspace_members WHERE workspace_id = $1 AND user_id = $2",
		workspaceID, userID,
	)
	if err != nil {
		var ownerID string
		err2 := h.db.Get(&ownerID, "SELECT owner_id FROM workspaces WHERE id = $1", workspaceID)
		if err2 != nil {
			return "", err2
		}
		if ownerID == userID {
			return "admin", nil
		}
		return "", err
	}
	return role, nil
}
