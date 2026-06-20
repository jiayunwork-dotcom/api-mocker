package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api-mocker/mock"
	"api-mocker/models"
)

func (h *Handler) HandleMock(c *gin.Context) {
	path := c.Param("path")

	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 2 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mock endpoint not found. Format: /mock/{project_id}/{api_path}"})
		return
	}

	projectID := segments[0]
	apiPath := "/" + strings.Join(segments[1:], "/")

	cacheKey := fmt.Sprintf("mock_api:%s:%s", projectID, apiPath)
	cached, err := h.rdb.Get(context.Background(), cacheKey).Result()
	if err == nil && cached != "" {
		var apiData models.API
		if json.Unmarshal([]byte(cached), &apiData) == nil {
			h.serveMock(c, &apiData, projectID)
			return
		}
	}

	var apiData models.API
	err = h.db.Get(&apiData,
		"SELECT * FROM apis WHERE project_id = $1 AND path = $2",
		projectID, apiPath,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Mock endpoint not found"})
		return
	}

	apiBytes, _ := json.Marshal(apiData)
	h.rdb.Set(context.Background(), cacheKey, string(apiBytes), 5*time.Minute)

	h.serveMock(c, &apiData, projectID)
}

func (h *Handler) serveMock(c *gin.Context, apiData *models.API, projectID string) {
	method := c.Request.Method
	if !strings.EqualFold(method, apiData.Method) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": fmt.Sprintf("Method %s not allowed, expected %s", method, apiData.Method)})
		return
	}

	var scenarios []models.MockScenario
	h.db.Select(&scenarios,
		"SELECT * FROM mock_scenarios WHERE api_id = $1 ORDER BY priority ASC",
		apiData.ID,
	)

	matcher := mock.NewMatcher()
	condCtx := h.buildConditionContext(c)

	for _, scenario := range scenarios {
		if len(scenario.Conditions) > 0 {
			conditionsJSON := []byte(scenario.Conditions)
			if matcher.Match(conditionsJSON, condCtx) {
				h.respondWithScenario(c, &scenario, projectID)
				return
			}
		}
	}

	h.respondWithDefault(c, apiData, projectID)
}

func (h *Handler) respondWithScenario(c *gin.Context, scenario *models.MockScenario, projectID string) {
	if scenario.DelayMs > 0 && scenario.DelayMs <= 200 {
		time.Sleep(time.Duration(scenario.DelayMs) * time.Millisecond)
	}

	var responseBody interface{}
	if len(scenario.Response) > 0 {
		json.Unmarshal(scenario.Response, &responseBody)
	}

	c.JSON(scenario.StatusCode, responseBody)
}

func (h *Handler) respondWithDefault(c *gin.Context, apiData *models.API, projectID string) {
	responsesRaw, _ := json.Marshal(apiData.Responses)
	var responses map[string]models.ResponseDef
	if err := json.Unmarshal(responsesRaw, &responses); err != nil {
		responses = map[string]models.ResponseDef{}
	}

	respDef, ok := responses["200"]
	if !ok {
		for _, v := range responses {
			respDef = v
			break
		}
	}

	generator := mock.NewGenerator(h.resolveModel)

	var result interface{}
	if len(respDef.Body) > 0 {
		result = generator.GenerateFromFields(projectID, respDef.Body)
	} else {
		result = map[string]interface{}{
			"message": "Mock response",
			"path":    apiData.Path,
			"method":  apiData.Method,
		}
	}

	c.JSON(200, result)
}

func (h *Handler) buildConditionContext(c *gin.Context) mock.ConditionContext {
	query := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string)
	for k, v := range c.Request.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	body := make(map[string]string)
	if c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			var bodyMap map[string]interface{}
			if json.Unmarshal(bodyBytes, &bodyMap) == nil {
				for k, v := range bodyMap {
					body[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	return mock.ConditionContext{
		Query:   query,
		Headers: headers,
		Body:    body,
	}
}

func (h *Handler) resolveModel(projectID, modelName string) ([]models.BodyField, error) {
	var model models.SharedModel
	err := h.db.Get(&model,
		"SELECT * FROM shared_models WHERE project_id = $1 AND name = $2",
		projectID, modelName,
	)
	if err != nil {
		return nil, err
	}

	schemaBytes, _ := json.Marshal(model.SchemaDefinition)
	return mock.ParseBodyFields(schemaBytes), nil
}

func (h *Handler) ListActivities(c *gin.Context) {
	projectID := c.Param("projectId")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	var versions []models.APIVersion
	err := h.db.Select(&versions, `
		SELECT av.*, u.name as changer_name
		FROM api_versions av
		JOIN users u ON av.changed_by = u.id
		JOIN apis a ON av.api_id = a.id
		WHERE a.project_id = $1
		ORDER BY av.created_at DESC
		LIMIT $2 OFFSET $3
	`, projectID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list activities"})
		return
	}

	var total int
	h.db.Get(&total, `
		SELECT COUNT(*) FROM api_versions av
		JOIN apis a ON av.api_id = a.id
		WHERE a.project_id = $1
	`, projectID)

	c.JSON(http.StatusOK, gin.H{
		"activities": versions,
		"total":      total,
		"page":       page,
		"pageSize":   pageSize,
	})
}
