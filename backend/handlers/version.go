package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"

	"api-mocker/models"
)

type DiffResult struct {
	Field    string      `json:"field"`
	Type     string      `json:"type"`
	OldValue interface{} `json:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty"`
}

func (h *Handler) createVersion(apiID, userID string, oldAPI *models.API, newAPI models.API) {
	snapshot := map[string]interface{}{
		"path":        newAPI.Path,
		"method":      newAPI.Method,
		"description": newAPI.Description,
		"params":      json.RawMessage(newAPI.Params),
		"requestBody": json.RawMessage(newAPI.RequestBody),
		"responses":   json.RawMessage(newAPI.Responses),
		"tags":        []string(newAPI.Tags),
	}

	snapshotJSON, _ := json.Marshal(snapshot)

	var oldSnapshot map[string]interface{}
	if oldAPI != nil {
		oldSnapshot = map[string]interface{}{
			"path":        oldAPI.Path,
			"method":      oldAPI.Method,
			"description": oldAPI.Description,
			"params":      json.RawMessage(oldAPI.Params),
			"requestBody": json.RawMessage(oldAPI.RequestBody),
			"responses":   json.RawMessage(oldAPI.Responses),
			"tags":        []string(oldAPI.Tags),
		}
	}

	var newSnapshot map[string]interface{}
	json.Unmarshal(snapshotJSON, &newSnapshot)

	summary := ""
	isBreaking := false

	if oldSnapshot != nil {
		diffs := computeDiff("", oldSnapshot, newSnapshot)
		var parts []string
		for _, d := range diffs {
			if d.Type == "removed" {
				isBreaking = true
				parts = append(parts, fmt.Sprintf("Removed %s", d.Field))
			} else if d.Type == "modified" {
				oldType := getType(d.OldValue)
				newType := getType(d.NewValue)
				if oldType != newType {
					isBreaking = true
				}
				parts = append(parts, fmt.Sprintf("Modified %s", d.Field))
			} else if d.Type == "added" {
				parts = append(parts, fmt.Sprintf("Added %s", d.Field))
			}
		}
		if len(parts) > 0 {
			summary = strings.Join(parts, "; ")
		} else {
			summary = "No changes detected"
		}
	} else {
		summary = "API created"
	}

	var maxVersion int
	h.db.Get(&maxVersion, "SELECT COALESCE(MAX(version), 0) FROM api_versions WHERE api_id = $1", apiID)

	_, err := h.db.Exec(
		`INSERT INTO api_versions (api_id, version, snapshot, change_summary, is_breaking, changed_by)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		apiID, maxVersion+1, string(snapshotJSON), summary, isBreaking, userID,
	)
	if err != nil {
		fmt.Printf("Failed to create version: %v\n", err)
	}
}

func (h *Handler) ListVersions(c *gin.Context) {
	apiID := c.Param("id")

	var versions []models.APIVersion
	err := h.db.Select(&versions, `
		SELECT av.*, u.name as changer_name
		FROM api_versions av
		JOIN users u ON av.changed_by = u.id
		WHERE av.api_id = $1
		ORDER BY av.version DESC
	`, apiID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *Handler) GetVersion(c *gin.Context) {
	versionID := c.Param("versionId")

	var version models.APIVersion
	err := h.db.Get(&version, `
		SELECT av.*, u.name as changer_name
		FROM api_versions av
		JOIN users u ON av.changed_by = u.id
		WHERE av.id = $1
	`, versionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"version": version})
}

func (h *Handler) DiffVersions(c *gin.Context) {
	versionID := c.Param("versionId")
	apiID := c.Param("id")

	var targetVersion models.APIVersion
	err := h.db.Get(&targetVersion, "SELECT * FROM api_versions WHERE id = $1 AND api_id = $2", versionID, apiID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	}

	if targetVersion.Version <= 1 {
		c.JSON(http.StatusOK, gin.H{"diffs": []DiffResult{}, "isBreaking": false})
		return
	}

	var prevVersion models.APIVersion
	err = h.db.Get(&prevVersion,
		"SELECT * FROM api_versions WHERE api_id = $1 AND version = $2",
		apiID, targetVersion.Version-1,
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"diffs": []DiffResult{}, "isBreaking": false})
		return
	}

	var oldSnap, newSnap map[string]interface{}
	json.Unmarshal([]byte(prevVersion.Snapshot), &oldSnap)
	json.Unmarshal([]byte(targetVersion.Snapshot), &newSnap)

	diffs := computeDiff("", oldSnap, newSnap)

	c.JSON(http.StatusOK, gin.H{
		"diffs":      diffs,
		"isBreaking": targetVersion.IsBreaking,
		"fromVersion": prevVersion.Version,
		"toVersion":  targetVersion.Version,
	})
}

func (h *Handler) RollbackVersion(c *gin.Context) {
	versionID := c.Param("versionId")
	apiID := c.Param("id")
	userID := c.GetString("userID")

	var version models.APIVersion
	err := h.db.Get(&version, "SELECT * FROM api_versions WHERE id = $1 AND api_id = $2", versionID, apiID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Version not found"})
		return
	}

	var snap map[string]interface{}
	json.Unmarshal([]byte(version.Snapshot), &snap)

	var oldAPI models.API
	h.db.Get(&oldAPI, "SELECT * FROM apis WHERE id = $1", apiID)

	paramsJSON, _ := json.Marshal(snap["params"])
	bodyJSON, _ := json.Marshal(snap["requestBody"])
	responsesJSON, _ := json.Marshal(snap["responses"])
	tags := []string{}
	if t, ok := snap["tags"].([]interface{}); ok {
		for _, v := range t {
			tags = append(tags, fmt.Sprintf("%v", v))
		}
	}

	_, err = h.db.Exec(
		`UPDATE apis SET path = $1, method = $2, description = $3, params = $4,
		 request_body = $5, responses = $6, tags = $7, updated_at = NOW() WHERE id = $8`,
		snap["path"], snap["method"], snap["description"],
		models.JSONB(paramsJSON), models.JSONB(bodyJSON), models.JSONB(responsesJSON), models.StringArray(tags), apiID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rollback"})
		return
	}

	var newAPI models.API
	h.db.Get(&newAPI, "SELECT * FROM apis WHERE id = $1", apiID)
	h.createVersion(apiID, userID, &oldAPI, newAPI)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Rolled back to version %d", version.Version), "api": newAPI})
}

func computeDiff(prefix string, old, new map[string]interface{}) []DiffResult {
	var diffs []DiffResult

	for key, oldVal := range old {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		newVal, exists := new[key]
		if !exists {
			diffs = append(diffs, DiffResult{Field: fullKey, Type: "removed", OldValue: oldVal})
			continue
		}

		if !reflect.DeepEqual(oldVal, newVal) {
			oldMap, oldIsMap := oldVal.(map[string]interface{})
			newMap, newIsMap := newVal.(map[string]interface{})
			if oldIsMap && newIsMap {
				diffs = append(diffs, computeDiff(fullKey, oldMap, newMap)...)
			} else {
				diffs = append(diffs, DiffResult{Field: fullKey, Type: "modified", OldValue: oldVal, NewValue: newVal})
			}
		}
	}

	for key, newVal := range new {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if _, exists := old[key]; !exists {
			diffs = append(diffs, DiffResult{Field: fullKey, Type: "added", NewValue: newVal})
		}
	}

	return diffs
}

func getType(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}
