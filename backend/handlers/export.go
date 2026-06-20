package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"api-mocker/models"
)

func (h *Handler) ExportOpenAPI(c *gin.Context) {
	var req struct {
		ProjectID string `json:"projectId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var project models.Project
	err := h.db.Get(&project, "SELECT * FROM projects WHERE id = $1", req.ProjectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	var apis []models.API
	h.db.Select(&apis, "SELECT * FROM apis WHERE project_id = $1 ORDER BY path ASC", req.ProjectID)

	var sharedModels []models.SharedModel
	h.db.Select(&sharedModels, "SELECT * FROM shared_models WHERE project_id = $1", req.ProjectID)

	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       project.Name,
			"description": project.Description,
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{"url": h.cfg.MockBaseURL + "/mock/" + project.ID, "description": "Mock Server"},
		},
		"paths":    buildPaths(apis),
		"components": map[string]interface{}{
			"schemas":         buildSchemas(sharedModels),
			"securitySchemes": map[string]interface{}{},
		},
	}

	c.JSON(http.StatusOK, doc)
}

func buildPaths(apis []models.API) map[string]interface{} {
	paths := make(map[string]interface{})
	for _, api := range apis {
		pathItem, exists := paths[api.Path]
		if !exists {
			pathItem = make(map[string]interface{})
		}
		pathMap := pathItem.(map[string]interface{})

		operation := map[string]interface{}{
			"summary":     api.Description,
			"operationId": fmt.Sprintf("%s_%s", strings.ToLower(api.Method), strings.ReplaceAll(strings.Trim(api.Path, "/"), "/", "_")),
			"tags":        api.Tags,
			"responses":   buildResponses(api),
		}

		var params []models.ParamField
		paramsRaw, _ := json.Marshal(api.Params)
		json.Unmarshal(paramsRaw, &params)
		if len(params) > 0 {
			operation["parameters"] = buildParameters(params)
		}

		var reqBody struct {
			Fields []models.BodyField `json:"fields"`
		}
		reqBodyRaw, _ := json.Marshal(api.RequestBody)
		json.Unmarshal(reqBodyRaw, &reqBody)
		if len(reqBody.Fields) > 0 {
			operation["requestBody"] = map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": buildSchemaFromFields(reqBody.Fields),
					},
				},
			}
		}

		pathMap[strings.ToLower(api.Method)] = operation
		paths[api.Path] = pathMap
	}
	return paths
}

func buildParameters(params []models.ParamField) []map[string]interface{} {
	var result []map[string]interface{}
	for _, p := range params {
		param := map[string]interface{}{
			"name":        p.Name,
			"in":          strings.ToLower(p.In),
			"required":    p.Required,
			"description": p.Desc,
			"schema": map[string]interface{}{
				"type": p.Type,
			},
		}
		if p.Example != "" {
			param["example"] = p.Example
		}
		result = append(result, param)
	}
	return result
}

func buildResponses(api models.API) map[string]interface{} {
	responses := make(map[string]interface{})

	var respDefs map[string]models.ResponseDef
	responsesRaw, _ := json.Marshal(api.Responses)
	json.Unmarshal(responsesRaw, &respDefs)

	for code, def := range respDefs {
		resp := map[string]interface{}{
			"description": def.Description,
		}
		if len(def.Body) > 0 {
			resp["content"] = map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": buildSchemaFromFields(def.Body),
				},
			}
		}
		responses[code] = resp
	}

	if _, ok := responses["200"]; !ok {
		responses["200"] = map[string]interface{}{
			"description": "Successful response",
		}
	}

	return responses
}

func buildSchemaFromFields(fields []models.BodyField) map[string]interface{} {
	properties := make(map[string]interface{})
	var required []string

	for _, f := range fields {
		if f.Required {
			required = append(required, f.Name)
		}

		prop := map[string]interface{}{}

		if f.Ref != "" {
			prop["$ref"] = "#/components/schemas/" + f.Ref
		} else if len(f.Enum) > 0 {
			prop["type"] = "string"
			prop["enum"] = f.Enum
		} else {
			switch f.Type {
			case "array":
				prop["type"] = "array"
				if len(f.Children) > 0 {
					prop["items"] = buildSchemaFromField(f.Children[0])
				} else {
					prop["items"] = map[string]interface{}{"type": "string"}
				}
			case "object":
				if len(f.Children) > 0 {
					prop["type"] = "object"
					childProps := make(map[string]interface{})
					for _, c := range f.Children {
						childProps[c.Name] = buildSchemaFromField(c)
					}
					prop["properties"] = childProps
				} else {
					prop["type"] = "object"
				}
			default:
				prop["type"] = f.Type
			}
		}

		if f.Desc != "" {
			prop["description"] = f.Desc
		}

		properties[f.Name] = prop
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func buildSchemaFromField(f models.BodyField) map[string]interface{} {
	prop := map[string]interface{}{}

	if f.Ref != "" {
		prop["$ref"] = "#/components/schemas/" + f.Ref
		return prop
	}

	if len(f.Enum) > 0 {
		prop["type"] = "string"
		prop["enum"] = f.Enum
		return prop
	}

	switch f.Type {
	case "array":
		prop["type"] = "array"
		if len(f.Children) > 0 {
			prop["items"] = buildSchemaFromField(f.Children[0])
		}
	case "object":
		prop["type"] = "object"
		if len(f.Children) > 0 {
			childProps := make(map[string]interface{})
			for _, c := range f.Children {
				childProps[c.Name] = buildSchemaFromField(c)
			}
			prop["properties"] = childProps
		}
	default:
		prop["type"] = f.Type
	}

	return prop
}

func buildSchemas(modelList []models.SharedModel) map[string]interface{} {
	schemas := make(map[string]interface{})
	for _, m := range modelList {
		var fields []models.BodyField
		schemaRaw, _ := json.Marshal(m.SchemaDefinition)
		json.Unmarshal(schemaRaw, &fields)
		schemas[m.Name] = buildSchemaFromFields(fields)
	}
	return schemas
}

func (h *Handler) ExportMarkdown(c *gin.Context) {
	var req struct {
		ProjectID string `json:"projectId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var project models.Project
	h.db.Get(&project, "SELECT * FROM projects WHERE id = $1", req.ProjectID)

	var apis []models.API
	h.db.Select(&apis, "SELECT * FROM apis WHERE project_id = $1 ORDER BY path ASC", req.ProjectID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", project.Name))
	if project.Description != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", project.Description))
	}
	sb.WriteString("## API Endpoints\n\n")

	for _, api := range apis {
		sb.WriteString(fmt.Sprintf("### %s %s\n\n", api.Method, api.Path))
		if api.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", api.Description))
		}

		var params []models.ParamField
		paramsRaw, _ := json.Marshal(api.Params)
		json.Unmarshal(paramsRaw, &params)
		if len(params) > 0 {
			sb.WriteString("**Parameters:**\n\n")
			sb.WriteString("| Name | In | Type | Required | Description |\n")
			sb.WriteString("|------|-----|------|----------|-------------|\n")
			for _, p := range params {
				req := "No"
				if p.Required {
					req = "Yes"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", p.Name, p.In, p.Type, req, p.Desc))
			}
			sb.WriteString("\n")
		}

		var reqBody struct {
			Fields []models.BodyField `json:"fields"`
		}
		reqBodyRaw, _ := json.Marshal(api.RequestBody)
		json.Unmarshal(reqBodyRaw, &reqBody)
		if len(reqBody.Fields) > 0 {
			sb.WriteString("**Request Body:**\n\n")
			sb.WriteString("| Name | Type | Required | Description |\n")
			sb.WriteString("|------|------|----------|-------------|\n")
			for _, f := range reqBody.Fields {
				req := "No"
				if f.Required {
					req = "Yes"
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", f.Name, f.Type, req, f.Desc))
			}
			sb.WriteString("\n")
		}

		var respDefs map[string]models.ResponseDef
		responsesRaw, _ := json.Marshal(api.Responses)
		json.Unmarshal(responsesRaw, &respDefs)

		sb.WriteString("**Responses:**\n\n")
		for code, def := range respDefs {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", code, def.Description))
		}
		sb.WriteString("\n---\n\n")
	}

	c.JSON(http.StatusOK, gin.H{"markdown": sb.String()})
}

func (h *Handler) GenerateCurl(c *gin.Context) {
	var req struct {
		APIID     string                 `json:"apiId" binding:"required"`
		BaseURL   string                 `json:"baseUrl"`
		Variables map[string]string      `json:"variables"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var api models.API
	err := h.db.Get(&api, "SELECT * FROM apis WHERE id = $1", req.APIID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "API not found"})
		return
	}

	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = h.cfg.MockBaseURL + "/mock/" + api.ProjectID
	}

	path := api.Path
	for k, v := range req.Variables {
		path = strings.ReplaceAll(path, ":"+k, v)
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}

	url := baseURL + path

	var parts []string
	parts = append(parts, fmt.Sprintf("curl -X %s '%s'", api.Method, url))

	var params []models.ParamField
	paramsRaw, _ := json.Marshal(api.Params)
	json.Unmarshal(paramsRaw, &params)
	for _, p := range params {
		if p.In == "header" && p.Example != "" {
			parts = append(parts, fmt.Sprintf("-H '%s: %s'", p.Name, p.Example))
		}
	}

	if api.Method != "GET" && api.Method != "HEAD" {
		parts = append(parts, "-H 'Content-Type: application/json'")
		var reqBody struct {
			Fields []models.BodyField `json:"fields"`
		}
		reqBodyRaw, _ := json.Marshal(api.RequestBody)
		json.Unmarshal(reqBodyRaw, &reqBody)
		if len(reqBody.Fields) > 0 {
			bodyMap := make(map[string]string)
			for _, f := range reqBody.Fields {
				val := f.Example
				if val == nil {
					val = fmt.Sprintf("<%s>", f.Type)
				}
				bodyMap[f.Name] = fmt.Sprintf("%v", val)
			}
			bodyJSON, _ := json.Marshal(bodyMap)
			parts = append(parts, fmt.Sprintf("-d '%s'", string(bodyJSON)))
		}
	}

	curlCmd := strings.Join(parts, " \\\n  ")
	c.JSON(http.StatusOK, gin.H{"curl": curlCmd})
}
