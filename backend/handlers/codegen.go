package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"api-mocker/codegen"
	"api-mocker/models"
)

func (h *Handler) GenerateCode(c *gin.Context) {
	var req struct {
		APIIDs   []string `json:"apiIds" binding:"required"`
		Language string   `json:"language" binding:"required,oneof=typescript go python"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	modelDefs, err := h.buildModelDefs(req.APIIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var code string
	switch req.Language {
	case "typescript":
		code = codegen.NewTypeScript().Generate(modelDefs)
	case "go":
		code = codegen.NewGo().Generate(modelDefs)
	case "python":
		code = codegen.NewPython().Generate(modelDefs)
	}

	c.JSON(http.StatusOK, gin.H{"code": code, "language": req.Language})
}

func (h *Handler) buildModelDefs(apiIDs []string) ([]codegen.ModelDef, error) {
	var result []codegen.ModelDef

	for _, apiID := range apiIDs {
		var api models.API
		err := h.db.Get(&api, "SELECT * FROM apis WHERE id = $1", apiID)
		if err != nil {
			continue
		}

		modelName := fmt.Sprintf("%s%s", strings.Title(strings.ToLower(api.Method)), pathToModelName(api.Path))

		var responses map[string]models.ResponseDef
		responsesRaw, _ := json.Marshal(api.Responses)
		json.Unmarshal(responsesRaw, &responses)

		if resp, ok := responses["200"]; ok && len(resp.Body) > 0 {
			result = append(result, codegen.ModelDef{
				Name:   modelName + "Response",
				Fields: codegen.ConvertBodyFields(resp.Body),
			})
		}

		var reqBody struct {
			Fields []models.BodyField `json:"fields"`
		}
		reqBodyRaw, _ := json.Marshal(api.RequestBody)
		json.Unmarshal(reqBodyRaw, &reqBody)
		if len(reqBody.Fields) > 0 {
			result = append(result, codegen.ModelDef{
				Name:   modelName + "Request",
				Fields: codegen.ConvertBodyFields(reqBody.Fields),
			})
		}
	}

	return result, nil
}

func pathToModelName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var nameParts []string
	for _, p := range parts {
		if strings.HasPrefix(p, ":") || strings.HasPrefix(p, "{") {
			p = strings.TrimPrefix(strings.TrimSuffix(p, "}"), ":")
			p = strings.TrimPrefix(strings.TrimSuffix(p, "}"), "{")
			p = "By" + strings.Title(strings.ToLower(p))
		}
		if p != "" {
			nameParts = append(nameParts, strings.Title(strings.ToLower(p)))
		}
	}
	return strings.Join(nameParts, "")
}
