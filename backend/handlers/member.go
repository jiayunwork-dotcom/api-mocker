package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

func (h *Handler) ListMembers(c *gin.Context) {
	workspaceID := c.Param("id")
	userID := c.GetString("userID")

	_, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to workspace"})
		return
	}

	var members []models.WorkspaceMember
	err = h.db.Select(&members, `
		SELECT wm.*, u.name as user_name, u.email as user_email
		FROM workspace_members wm
		JOIN users u ON wm.user_id = u.id
		WHERE wm.workspace_id = $1
		ORDER BY wm.joined_at ASC
	`, workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

func (h *Handler) InviteMember(c *gin.Context) {
	workspaceID := c.Param("id")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role" binding:"required,oneof=admin editor viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var memberCount int
	h.db.Get(&memberCount, "SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1", workspaceID)
	if memberCount >= 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Workspace member limit (100) reached"})
		return
	}

	invitation := models.Invitation{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Email:       req.Email,
		Role:        req.Role,
		Token:       uuid.New().String(),
		InvitedBy:   userID,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}

	_, err = h.db.Exec(
		"INSERT INTO invitations (id, workspace_id, email, role, token, invited_by, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		invitation.ID, invitation.WorkspaceID, invitation.Email, invitation.Role,
		invitation.Token, invitation.InvitedBy, invitation.ExpiresAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invitation"})
		return
	}

	inviteLink := fmt.Sprintf("/join?token=%s", invitation.Token)

	c.JSON(http.StatusCreated, gin.H{
		"invitation":  invitation,
		"invite_link": inviteLink,
	})
}

func (h *Handler) JoinWorkspace(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		Token string `json:"token" binding:"required"`
		Role  string `json:"role" binding:"required,oneof=admin editor viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var invitation models.Invitation
	err := h.db.Get(&invitation, "SELECT * FROM invitations WHERE token = $1 AND accepted_at IS NULL", req.Token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired invitation"})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invitation has expired"})
		return
	}

	tx, err := h.db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}
	defer tx.Rollback()

	var existing int
	tx.Get(&existing, "SELECT COUNT(*) FROM workspace_members WHERE workspace_id = $1 AND user_id = $2",
		invitation.WorkspaceID, userID)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Already a member of this workspace"})
		return
	}

	_, err = tx.Exec(
		"INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)",
		invitation.WorkspaceID, userID, req.Role,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join workspace"})
		return
	}

	_, err = tx.Exec(
		"UPDATE invitations SET accepted_at = NOW() WHERE id = $1",
		invitation.ID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update invitation"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Joined workspace successfully",
		"workspace_id": invitation.WorkspaceID,
		"role":         req.Role,
	})
}

func (h *Handler) UpdateMemberRole(c *gin.Context) {
	workspaceID := c.Param("id")
	memberID := c.Param("memberId")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required,oneof=admin editor viewer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.db.Exec(
		"UPDATE workspace_members SET role = $1 WHERE id = $2 AND workspace_id = $3",
		req.Role, memberID, workspaceID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update member role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member role updated"})
}

func (h *Handler) RemoveMember(c *gin.Context) {
	workspaceID := c.Param("id")
	memberID := c.Param("memberId")
	userID := c.GetString("userID")

	role, err := h.getWorkspaceRole(workspaceID, userID)
	if err != nil || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		return
	}

	var targetMember models.WorkspaceMember
	err = h.db.Get(&targetMember, "SELECT * FROM workspace_members WHERE id = $1 AND workspace_id = $2", memberID, workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return
	}

	var workspace models.Workspace
	h.db.Get(&workspace, "SELECT * FROM workspaces WHERE id = $1", workspaceID)
	if targetMember.UserID == workspace.OwnerID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove workspace owner"})
		return
	}

	_, err = h.db.Exec("DELETE FROM workspace_members WHERE id = $1 AND workspace_id = $2", memberID, workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}
