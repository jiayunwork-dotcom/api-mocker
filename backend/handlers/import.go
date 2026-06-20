package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"api-mocker/models"
)

const maxImportFileSize = 2 * 1024 * 1024

type ImportRequest struct {
	Content string `json:"content"`
}

type ImportResultItem struct {
	Path   string `json:"path"`
	Method string `json:"method"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ImportResult struct {
	Success int                 `json:"success"`
	Skipped int                 `json:"skipped"`
	Failed  int                 `json:"failed"`
	Items   []ImportResultItem  `json:"items"`
}

func (h *Handler) ImportOpenAPI(c *gin.Context) {
	projectID := c.Param("projectId")
	userID := c.GetString("userID")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	if c.Request.ContentLength > maxImportFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("文件大小超过限制，最大允许 %d MB", maxImportFileSize/1024/1024),
		})
		return
	}

	var content string
	contentType := c.ContentType()

	if strings.Contains(contentType, "multipart/form-data") {
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get uploaded file"})
			return
		}
		defer file.Close()

		if header.Size > maxImportFileSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("文件大小超过限制，最大允许 %d MB", maxImportFileSize/1024/1024),
			})
			return
		}

		fileBytes, err := io.ReadAll(file)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("文件大小超过限制，最大允许 %d MB", maxImportFileSize/1024/1024),
				})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read file content"})
			}
			return
		}

		content = string(fileBytes)
	} else {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("请求内容超过限制，最大允许 %d MB", maxImportFileSize/1024/1024),
				})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			}
			return
		}
		c.Request.Body.Close()

		var req ImportRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}
		content = req.Content
	}

	if strings.TrimSpace(content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content cannot be empty"})
		return
	}

	openapi, err := parseOpenAPI(content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.importAPIs(projectID, userID, openapi)

	c.JSON(http.StatusOK, gin.H{"result": result})
}

func parseOpenAPI(content string) (map[string]interface{}, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return nil, fmt.Errorf("JSON 格式错误: %v", err)
	}

	openapiVersion, ok := doc["openapi"].(string)
	if !ok {
		return nil, fmt.Errorf("不是有效的 OpenAPI 文档: 缺少 openapi 字段")
	}
	if !strings.HasPrefix(openapiVersion, "3.") {
		return nil, fmt.Errorf("不支持的 OpenAPI 版本: %s, 需要 3.0.x", openapiVersion)
	}

	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("OpenAPI 文档缺少 paths 字段")
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths 对象为空，没有可导入的接口")
	}

	return doc, nil
}

func (h *Handler) importAPIs(projectID, userID string, openapi map[string]interface{}) ImportResult {
	result := ImportResult{
		Items: []ImportResultItem{},
	}

	paths := openapi["paths"].(map[string]interface{})

	var apiCount int
	h.db.Get(&apiCount, "SELECT COUNT(*) FROM apis WHERE project_id = $1", projectID)

	validMethods := map[string]bool{
		"get": true, "post": true, "put": true, "patch": true,
		"delete": true, "head": true, "options": true,
	}

	for path, pathItem := range paths {
		pathItemMap, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}

		for method, operation := range pathItemMap {
			methodLower := strings.ToLower(method)
			if !validMethods[methodLower] {
				continue
			}

			operationMap, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}

			item := ImportResultItem{
				Path:   path,
				Method: strings.ToUpper(methodLower),
			}

			if !isValidPath(path) {
				item.Status = "failed"
				item.Error = fmt.Sprintf("路径格式无效: %s", path)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			var existing int
			h.db.Get(&existing,
				"SELECT COUNT(*) FROM apis WHERE project_id = $1 AND path = $2 AND method = $3",
				projectID, path, strings.ToUpper(methodLower),
			)
			if existing > 0 {
				item.Status = "skipped"
				item.Error = "接口已存在"
				result.Skipped++
				result.Items = append(result.Items, item)
				continue
			}

			if apiCount >= 500 {
				item.Status = "failed"
				item.Error = "项目 API 数量已达上限 (500)"
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			api, err := convertOpenAPIOperation(path, methodLower, operationMap, openapi)
			if err != nil {
				item.Status = "failed"
				item.Error = err.Error()
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			api.ID = uuid.New().String()
			api.ProjectID = projectID

			tx, err := h.db.Beginx()
			if err != nil {
				item.Status = "failed"
				item.Error = "数据库事务启动失败"
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			_, err = tx.Exec(
				`INSERT INTO apis (id, project_id, path, method, description, params, request_body, responses, tags)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				api.ID, api.ProjectID, api.Path, api.Method, api.Description,
				api.Params, api.RequestBody, api.Responses, api.Tags,
			)
			if err != nil {
				tx.Rollback()
				item.Status = "failed"
				item.Error = fmt.Sprintf("保存失败: %v", err)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			snapshot := map[string]interface{}{
				"path":        api.Path,
				"method":      api.Method,
				"description": api.Description,
				"params":      json.RawMessage(api.Params),
				"requestBody": json.RawMessage(api.RequestBody),
				"responses":   json.RawMessage(api.Responses),
				"tags":        []string(api.Tags),
			}
			snapshotJSON, _ := json.Marshal(snapshot)

			_, err = tx.Exec(
				`INSERT INTO api_versions (api_id, version, snapshot, change_summary, is_breaking, changed_by)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				api.ID, 1, string(snapshotJSON), "API created (imported)", false, userID,
			)
			if err != nil {
				tx.Rollback()
				item.Status = "failed"
				item.Error = fmt.Sprintf("创建版本失败: %v", err)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			if err := tx.Commit(); err != nil {
				item.Status = "failed"
				item.Error = fmt.Sprintf("提交事务失败: %v", err)
				result.Failed++
				result.Items = append(result.Items, item)
				continue
			}

			apiCount++
			item.Status = "success"
			result.Success++
			result.Items = append(result.Items, item)
		}
	}

	return result
}

func convertOpenAPIOperation(path, method string, operation map[string]interface{}, openapi map[string]interface{}) (models.API, error) {
	api := models.API{
		Path:   path,
		Method: strings.ToUpper(method),
		Tags:   models.StringArray{},
	}

	if desc, ok := operation["description"].(string); ok {
		api.Description = desc
	} else if summary, ok := operation["summary"].(string); ok {
		api.Description = summary
	}

	if tags, ok := operation["tags"].([]interface{}); ok {
		for _, t := range tags {
			if tagStr, ok := t.(string); ok {
				api.Tags = append(api.Tags, tagStr)
			}
		}
	}

	params, err := convertParameters(operation["parameters"])
	if err != nil {
		return api, fmt.Errorf("解析 parameters 失败: %v", err)
	}
	paramsJSON, _ := json.Marshal(params)
	api.Params = models.JSONB(paramsJSON)

	reqBodyFields, err := convertRequestBody(operation["requestBody"], openapi)
	if err != nil {
		return api, fmt.Errorf("解析 requestBody 失败: %v", err)
	}
	reqBodyJSON, _ := json.Marshal(map[string]interface{}{"fields": reqBodyFields})
	api.RequestBody = models.JSONB(reqBodyJSON)

	responses, err := convertResponses(operation["responses"], openapi)
	if err != nil {
		return api, fmt.Errorf("解析 responses 失败: %v", err)
	}
	responsesJSON, _ := json.Marshal(responses)
	api.Responses = models.JSONB(responsesJSON)

	return api, nil
}

func convertParameters(paramsRaw interface{}) ([]models.ParamField, error) {
	params := []models.ParamField{}
	if paramsRaw == nil {
		return params, nil
	}

	paramsList, ok := paramsRaw.([]interface{})
	if !ok {
		return params, nil
	}

	for _, p := range paramsList {
		paramMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		param := models.ParamField{}

		if name, ok := paramMap["name"].(string); ok {
			param.Name = name
		} else {
			continue
		}

		if in, ok := paramMap["in"].(string); ok {
			param.In = in
		} else {
			continue
		}

		if required, ok := paramMap["required"].(bool); ok {
			param.Required = required
		}

		if desc, ok := paramMap["description"].(string); ok {
			param.Desc = desc
		}

		if example, ok := paramMap["example"]; ok {
			param.Example = fmt.Sprintf("%v", example)
		}

		if schema, ok := paramMap["schema"].(map[string]interface{}); ok {
			if schemaType, ok := schema["type"].(string); ok {
				param.Type = schemaType
			}
		}

		if param.Type == "" {
			param.Type = "string"
		}

		params = append(params, param)
	}

	return params, nil
}

func convertRequestBody(reqBodyRaw interface{}, openapi map[string]interface{}) ([]models.BodyField, error) {
	fields := []models.BodyField{}
	if reqBodyRaw == nil {
		return fields, nil
	}

	reqBodyMap, ok := reqBodyRaw.(map[string]interface{})
	if !ok {
		return fields, nil
	}

	content, ok := reqBodyMap["content"].(map[string]interface{})
	if !ok {
		return fields, nil
	}

	var jsonContent map[string]interface{}
	for k, v := range content {
		if strings.Contains(k, "json") {
			jsonContent, _ = v.(map[string]interface{})
			break
		}
	}
	if jsonContent == nil {
		return fields, nil
	}

	schema, ok := jsonContent["schema"].(map[string]interface{})
	if !ok {
		return fields, nil
	}

	resolvedSchema := resolveSchemaRef(schema, openapi)
	return convertSchemaToBodyFields(resolvedSchema, openapi), nil
}

func convertResponses(responsesRaw interface{}, openapi map[string]interface{}) (map[string]models.ResponseDef, error) {
	result := make(map[string]models.ResponseDef)
	if responsesRaw == nil {
		return result, nil
	}

	responsesMap, ok := responsesRaw.(map[string]interface{})
	if !ok {
		return result, nil
	}

	for code, resp := range responsesMap {
		respMap, ok := resp.(map[string]interface{})
		if !ok {
			continue
		}

		respDef := models.ResponseDef{}

		if desc, ok := respMap["description"].(string); ok {
			respDef.Description = desc
		}

		if content, ok := respMap["content"].(map[string]interface{}); ok {
			var jsonContent map[string]interface{}
			for k, v := range content {
				if strings.Contains(k, "json") {
					jsonContent, _ = v.(map[string]interface{})
					break
				}
			}
			if jsonContent != nil {
				if schema, ok := jsonContent["schema"].(map[string]interface{}); ok {
					resolvedSchema := resolveSchemaRef(schema, openapi)
					respDef.Body = convertSchemaToBodyFields(resolvedSchema, openapi)
				}
			}
		}

		result[code] = respDef
	}

	if _, ok := result["200"]; !ok {
		result["200"] = models.ResponseDef{
			Description: "Successful response",
		}
	}

	return result, nil
}

func resolveSchemaRef(schema map[string]interface{}, openapi map[string]interface{}) map[string]interface{} {
	if ref, ok := schema["$ref"].(string); ok {
		parts := strings.Split(ref, "/")
		if len(parts) == 4 && parts[0] == "#" {
			if components, ok := openapi[parts[1]].(map[string]interface{}); ok {
				if schemas, ok := components[parts[2]].(map[string]interface{}); ok {
					if resolved, ok := schemas[parts[3]].(map[string]interface{}); ok {
						return resolved
					}
				}
			}
		}
	}
	return schema
}

func convertSchemaToBodyFields(schema map[string]interface{}, openapi map[string]interface{}) []models.BodyField {
	fields := []models.BodyField{}

	schema = resolveSchemaRef(schema, openapi)

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return fields
	}

	requiredList := []string{}
	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			if rStr, ok := r.(string); ok {
				requiredList = append(requiredList, rStr)
			}
		}
	}

	requiredMap := make(map[string]bool)
	for _, r := range requiredList {
		requiredMap[r] = true
	}

	for name, prop := range properties {
		propMap, ok := prop.(map[string]interface{})
		if !ok {
			continue
		}

		propMap = resolveSchemaRef(propMap, openapi)

		field := models.BodyField{
			Name:     name,
			Required: requiredMap[name],
		}

		if typeStr, ok := propMap["type"].(string); ok {
			field.Type = typeStr
		}

		if desc, ok := propMap["description"].(string); ok {
			field.Desc = desc
		}

		if example, ok := propMap["example"]; ok {
			field.Example = example
		}

		if enum, ok := propMap["enum"].([]interface{}); ok {
			for _, e := range enum {
				if eStr, ok := e.(string); ok {
					field.Enum = append(field.Enum, eStr)
				}
			}
		}

		if field.Type == "object" {
			field.Children = convertSchemaToBodyFields(propMap, openapi)
		}

		if field.Type == "array" {
			if items, ok := propMap["items"].(map[string]interface{}); ok {
				items = resolveSchemaRef(items, openapi)
				itemFields := convertSchemaToBodyFields(items, openapi)
				if len(itemFields) > 0 {
					field.Children = itemFields
				} else {
					if itemType, ok := items["type"].(string); ok {
						field.Children = []models.BodyField{{
							Name: "",
							Type: itemType,
						}}
					}
				}
			}
		}

		fields = append(fields, field)
	}

	return fields
}
