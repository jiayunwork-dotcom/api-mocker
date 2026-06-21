package handlers

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"api-mocker/models"
)

type flatField struct {
	path string
	name string
	typ  string
}

func flattenFields(fields []models.BodyField, prefix string) []flatField {
	var result []flatField
	for _, f := range fields {
		path := f.Name
		if prefix != "" {
			path = prefix + "." + f.Name
		}
		result = append(result, flatField{path: path, name: f.Name, typ: f.Type})

		if f.Type == "array" && len(f.Children) > 0 {
			for _, child := range f.Children {
				childPath := path + "[*]." + child.Name
				result = append(result, flatField{path: childPath, name: child.Name, typ: child.Type})
				if len(child.Children) > 0 {
					result = append(result, flattenFields(child.Children, childPath)...)
				}
			}
		} else if len(f.Children) > 0 {
			result = append(result, flattenFields(f.Children, path)...)
		}
	}
	return result
}

func extractResponseBody(responsesJSON models.JSONB) []models.BodyField {
	var responses map[string]models.ResponseDef
	if err := json.Unmarshal([]byte(responsesJSON), &responses); err != nil {
		return nil
	}

	for _, resp := range responses {
		if len(resp.Body) > 0 {
			return resp.Body
		}
	}
	return nil
}

type fieldChange struct {
	changeType string
	fieldPath  string
	oldField   *flatField
	newField   *flatField
}

func compareResponseFields(oldResp, newResp models.JSONB) []fieldChange {
	oldFields := flattenFields(extractResponseBody(oldResp), "")
	newFields := flattenFields(extractResponseBody(newResp), "")

	oldFieldMap := make(map[string]flatField)
	for _, f := range oldFields {
		oldFieldMap[f.path] = f
	}

	newFieldMap := make(map[string]flatField)
	for _, f := range newFields {
		newFieldMap[f.path] = f
	}

	var changes []fieldChange

	for path, oldF := range oldFieldMap {
		if _, exists := newFieldMap[path]; !exists {
			changes = append(changes, fieldChange{
				changeType: "delete",
				fieldPath:  path,
				oldField:   &oldF,
			})
		}
	}

	for path, newF := range newFieldMap {
		if oldF, exists := oldFieldMap[path]; exists {
			if oldF.typ != newF.typ {
				changes = append(changes, fieldChange{
					changeType: "type_change",
					fieldPath:  path,
					oldField:   &oldF,
					newField:   &newF,
				})
			}
		}
	}

	deletedPaths := make(map[string]bool)
	for _, ch := range changes {
		if ch.changeType == "delete" {
			deletedPaths[ch.fieldPath] = true
		}
	}

	matchedNewPaths := make(map[string]bool)
	for _, oldF := range oldFields {
		if deletedPaths[oldF.path] {
			continue
		}
		if _, exists := newFieldMap[oldF.path]; !exists {
			parentPath := ""
			dotIdx := strings.LastIndex(oldF.path, ".")
			if dotIdx >= 0 {
				parentPath = oldF.path[:dotIdx]
			}

			for _, newF := range newFields {
				if _, exists := oldFieldMap[newF.path]; exists {
					continue
				}
				if matchedNewPaths[newF.path] {
					continue
				}

				newParentPath := ""
				newDotIdx := strings.LastIndex(newF.path, ".")
				if newDotIdx >= 0 {
					newParentPath = newF.path[:newDotIdx]
				}

				if parentPath == newParentPath && oldF.typ == newF.typ && oldF.name != newF.name {
					changes = append(changes, fieldChange{
						changeType: "rename",
						fieldPath:  oldF.path,
						oldField:   &oldF,
						newField:   &newF,
					})
					matchedNewPaths[newF.path] = true
					break
				}
			}
		}
	}

	return changes
}

func (h *Handler) analyzeImpact(apiID, projectID, userID string, oldAPI, newAPI models.API) (*models.ImpactReport, error) {
	changes := compareResponseFields(oldAPI.Responses, newAPI.Responses)
	if len(changes) == 0 {
		return nil, nil
	}

	var deps []models.APIDependency
	err := h.db.Select(&deps, `
		SELECT 
			d.*,
			ua.path AS upstream_path,
			ua.method AS upstream_method,
			da.path AS downstream_path,
			da.method AS downstream_method
		FROM api_dependencies d
		INNER JOIN apis ua ON d.upstream_api_id = ua.id
		INNER JOIN apis da ON d.downstream_api_id = da.id
		WHERE d.upstream_api_id = $1
	`, apiID)
	if err != nil {
		return nil, err
	}

	if len(deps) == 0 {
		return nil, nil
	}

	var changedFields []models.ChangedField
	var affectedDownstream []models.AffectedDownstream
	hasBreakingChange := false

	for _, ch := range changes {
		cf := models.ChangedField{
			FieldPath:  ch.fieldPath,
			ChangeType: ch.changeType,
		}
		if ch.oldField != nil {
			cf.OldType = ch.oldField.typ
			cf.OldName = ch.oldField.name
		}
		if ch.newField != nil {
			cf.NewType = ch.newField.typ
			cf.NewName = ch.newField.name
		}
		changedFields = append(changedFields, cf)
	}

	for _, dep := range deps {
		var mappings []models.FieldMapping
		json.Unmarshal([]byte(dep.FieldMappings), &mappings)

		var affectedMappings []string
		impactLevel := ""

		for _, mapping := range mappings {
			for _, ch := range changes {
				if mapping.UpstreamField == ch.fieldPath ||
				   (ch.oldField != nil && mapping.UpstreamField == ch.oldField.path) ||
				   (ch.newField != nil && mapping.UpstreamField == ch.newField.path) {

					affectedMappings = append(affectedMappings,
						mapping.UpstreamField+" -> "+mapping.DownstreamField)

					if ch.changeType == "delete" || ch.changeType == "type_change" {
						impactLevel = "Breaking"
						hasBreakingChange = true
					} else if ch.changeType == "rename" && impactLevel != "Breaking" {
						impactLevel = "Warning"
					}
				}
			}
		}

		if len(affectedMappings) > 0 {
			affectedDownstream = append(affectedDownstream, models.AffectedDownstream{
				DownstreamAPIID:  dep.DownstreamAPIID,
				DownstreamPath:   dep.DownstreamPath,
				DownstreamMethod: dep.DownstreamMethod,
				AffectedMappings: affectedMappings,
				ImpactLevel:      impactLevel,
			})
		}
	}

	if len(affectedDownstream) == 0 {
		return nil, nil
	}

	changeType := "mixed"
	if len(changes) > 0 {
		allDelete := true
		allType := true
		allRename := true
		for _, ch := range changes {
			if ch.changeType != "delete" {
				allDelete = false
			}
			if ch.changeType != "type_change" {
				allType = false
			}
			if ch.changeType != "rename" {
				allRename = false
			}
		}
		if allDelete {
			changeType = "field_delete"
		} else if allType {
			changeType = "type_change"
		} else if allRename {
			changeType = "field_rename"
		}
	}

	changedFieldsJSON, _ := json.Marshal(changedFields)
	affectedJSON, _ := json.Marshal(affectedDownstream)

	report := models.ImpactReport{
		ID:                uuid.New().String(),
		ProjectID:         projectID,
		ChangedAPIID:      apiID,
		ChangedAPIPath:    newAPI.Path,
		ChangedAPIMethod:  newAPI.Method,
		ChangeType:        changeType,
		ChangedFields:     models.JSONB(changedFieldsJSON),
		AffectedDownstream: models.JSONB(affectedJSON),
		HasBreakingChange: hasBreakingChange,
		CreatedBy:         userID,
	}

	_, err = h.db.Exec(`
		INSERT INTO impact_reports (
			id, project_id, changed_api_id, changed_api_path, changed_api_method,
			change_type, changed_fields, affected_downstream, has_breaking_change, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, report.ID, report.ProjectID, report.ChangedAPIID, report.ChangedAPIPath,
		report.ChangedAPIMethod, report.ChangeType, report.ChangedFields,
		report.AffectedDownstream, report.HasBreakingChange, report.CreatedBy)
	if err != nil {
		return nil, err
	}

	return &report, nil
}

func (h *Handler) BroadcastDependencyBreak(projectID string, msg models.DependencyBreakMessage) {
	msg.EventType = "dependency_break"
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.wsHub.BroadcastToProject(projectID, data)
}
