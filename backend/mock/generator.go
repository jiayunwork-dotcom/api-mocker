package mock

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"api-mocker/models"
)

type Generator struct {
	modelResolver func(projectID, modelName string) ([]models.BodyField, error)
}

func NewGenerator(modelResolver func(projectID, modelName string) ([]models.BodyField, error)) *Generator {
	return &Generator{modelResolver: modelResolver}
}

func (g *Generator) GenerateFromFields(projectID string, fields []models.BodyField) map[string]interface{} {
	result := make(map[string]interface{})
	for _, field := range fields {
		result[field.Name] = g.generateField(projectID, field)
	}
	return result
}

func (g *Generator) generateField(projectID string, field models.BodyField) interface{} {
	if field.Ref != "" {
		if g.modelResolver != nil {
			refFields, err := g.modelResolver(projectID, field.Ref)
			if err == nil {
				return g.GenerateFromFields(projectID, refFields)
			}
		}
		return map[string]interface{}{}
	}

	switch field.Type {
	case "string":
		return g.generateString(field.Name, field.Enum)
	case "number", "integer":
		return g.generateNumber(field.Type)
	case "boolean":
		return rand.Intn(2) == 1
	case "array":
		if len(field.Children) > 0 {
			count := rand.Intn(5) + 1
			arr := make([]interface{}, count)
			for i := 0; i < count; i++ {
				arr[i] = g.generateField(projectID, field.Children[0])
			}
			return arr
		}
		count := rand.Intn(5) + 1
		arr := make([]string, count)
		for i := 0; i < count; i++ {
			arr[i] = g.generateString(field.Name, nil)
		}
		return arr
	case "object":
		if len(field.Children) > 0 {
			return g.GenerateFromFields(projectID, field.Children)
		}
		return map[string]interface{}{}
	default:
		return nil
	}
}

func (g *Generator) generateString(name string, enum []string) string {
	if len(enum) > 0 {
		return enum[rand.Intn(len(enum))]
	}

	lower := strings.ToLower(name)

	if strings.Contains(lower, "email") || strings.Contains(lower, "mail") {
		return fmt.Sprintf("user%d@example.com", rand.Intn(10000))
	}
	if strings.Contains(lower, "phone") || strings.Contains(lower, "mobile") || strings.Contains(lower, "tel") {
		return fmt.Sprintf("1%d%04d%04d", 30+rand.Intn(10), rand.Intn(10000), rand.Intn(10000))
	}
	if strings.Contains(lower, "name") && !strings.Contains(lower, "file") && !strings.Contains(lower, "user") {
		firstNames := []string{"张伟", "李娜", "王芳", "刘洋", "陈明", "赵磊", "孙丽", "周强", "吴敏", "郑华"}
		return firstNames[rand.Intn(len(firstNames))]
	}
	if strings.Contains(lower, "username") || strings.Contains(lower, "user_name") {
		return fmt.Sprintf("user_%d", rand.Intn(10000))
	}
	if strings.Contains(lower, "url") || strings.Contains(lower, "link") || strings.Contains(lower, "website") {
		domains := []string{"example.com", "api.test.io", "demo.dev", "mock.service"}
		paths := []string{"/api/v1", "/users", "/data", "/items", "/posts"}
		return fmt.Sprintf("https://%s%s", domains[rand.Intn(len(domains))], paths[rand.Intn(len(paths))])
	}
	if strings.Contains(lower, "avatar") || strings.Contains(lower, "photo") || strings.Contains(lower, "image") || strings.Contains(lower, "img") {
		sizes := []string{"200", "300", "400"}
		return fmt.Sprintf("https://i.pravatar.cc/%s?img=%d", sizes[rand.Intn(len(sizes))], rand.Intn(70)+1)
	}
	if strings.Contains(lower, "address") || strings.Contains(lower, "addr") {
		cities := []string{"北京市朝阳区", "上海市浦东新区", "广州市天河区", "深圳市南山区", "杭州市西湖区"}
		streets := []string{"中山路", "人民路", "建设路", "和平路", "解放路"}
		return fmt.Sprintf("%s%s%d号", cities[rand.Intn(len(cities))], streets[rand.Intn(len(streets))], rand.Intn(200)+1)
	}
	if strings.Contains(lower, "id") || strings.Contains(lower, "_id") {
		return fmt.Sprintf("%d", rand.Intn(100000))
	}
	if strings.Contains(lower, "date") || strings.Contains(lower, "time") || strings.Contains(lower, "at") {
		return time.Now().Add(time.Duration(rand.Intn(365*24)) * time.Hour).Format("2006-01-02T15:04:05Z")
	}
	if strings.Contains(lower, "color") || strings.Contains(lower, "colour") {
		colors := []string{"#FF5733", "#33FF57", "#3357FF", "#F033FF", "#FF33A8", "#33FFF0"}
		return colors[rand.Intn(len(colors))]
	}
	if strings.Contains(lower, "ip") {
		return fmt.Sprintf("192.168.%d.%d", rand.Intn(256), rand.Intn(256))
	}
	if strings.Contains(lower, "status") {
		statuses := []string{"active", "inactive", "pending", "archived"}
		return statuses[rand.Intn(len(statuses))]
	}
	if strings.Contains(lower, "token") || strings.Contains(lower, "key") {
		return fmt.Sprintf("tk_%016x", rand.Int63())
	}
	if strings.Contains(lower, "desc") || strings.Contains(lower, "description") || strings.Contains(lower, "content") || strings.Contains(lower, "text") {
		texts := []string{
			"这是一段示例文本内容",
			"用于演示的模拟数据",
			"自动生成的描述信息",
			"测试用的文字内容",
		}
		return texts[rand.Intn(len(texts))]
	}

	return fmt.Sprintf("string_%d", rand.Intn(10000))
}

func (g *Generator) generateNumber(typeName string) interface{} {
	if typeName == "integer" {
		return rand.Intn(10000)
	}
	return float64(rand.Intn(10000)) / 100.0
}

func ParseBodyFields(raw json.RawMessage) []models.BodyField {
	var fields []models.BodyField
	if err := json.Unmarshal(raw, &fields); err != nil {
		return []models.BodyField{}
	}
	return fields
}
