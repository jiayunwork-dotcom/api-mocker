package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"

	"api-mocker/models"
	ws "api-mocker/websocket"
)

func (h *Handler) ListProbes(c *gin.Context) {
	projectID := c.Param("projectId")
	groupName := c.Query("groupName")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var probes []models.ProbeConfig
	var err error

	if groupName != "" {
		err = h.db.Select(&probes,
			"SELECT * FROM probe_configs WHERE project_id = $1 AND group_name = $2 ORDER BY created_at ASC",
			projectID, groupName,
		)
	} else {
		err = h.db.Select(&probes,
			"SELECT * FROM probe_configs WHERE project_id = $1 ORDER BY created_at ASC",
			projectID,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list probes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"probes": probes})
}

func (h *Handler) CreateProbe(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		APIID            string `json:"apiId" binding:"required"`
		Enabled          bool   `json:"enabled"`
		GroupName        string `json:"groupName"`
		IntervalSeconds  int    `json:"intervalSeconds"`
		TimeoutMs        int    `json:"timeoutMs"`
		FailThreshold    int    `json:"failThreshold"`
		RecoverThreshold int    `json:"recoverThreshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var api models.API
	err = h.db.Get(&api, "SELECT * FROM apis WHERE id = $1 AND project_id = $2", req.APIID, projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API not found in this project"})
		return
	}

	var existing int
	h.db.Get(&existing,
		"SELECT COUNT(*) FROM probe_configs WHERE api_id = $1",
		req.APIID,
	)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Probe already exists for this API"})
		return
	}

	if req.Enabled {
		var enabledCount int
		h.db.Get(&enabledCount,
			"SELECT COUNT(*) FROM probe_configs WHERE project_id = $1 AND enabled = true",
			projectID,
		)
		if enabledCount >= 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 20 enabled probes per project"})
			return
		}
	}

	intervalSeconds := req.IntervalSeconds
	if intervalSeconds < 10 {
		intervalSeconds = 30
	}
	if intervalSeconds > 300 {
		intervalSeconds = 300
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 3000
	}

	failThreshold := req.FailThreshold
	if failThreshold <= 0 {
		failThreshold = 3
	}

	recoverThreshold := req.RecoverThreshold
	if recoverThreshold <= 0 {
		recoverThreshold = 2
	}

	probe := models.ProbeConfig{
		ID:                   uuid.New().String(),
		APIID:                req.APIID,
		ProjectID:            projectID,
		Enabled:              req.Enabled,
		GroupName:            req.GroupName,
		IntervalSeconds:      intervalSeconds,
		TimeoutMs:            timeoutMs,
		FailThreshold:        failThreshold,
		RecoverThreshold:     recoverThreshold,
		Status:               "healthy",
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 0,
	}

	_, err = h.db.Exec(
		`INSERT INTO probe_configs (id, api_id, project_id, enabled, group_name, interval_seconds, timeout_ms, fail_threshold, recover_threshold, status, consecutive_failures, consecutive_successes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		probe.ID, probe.APIID, probe.ProjectID, probe.Enabled, probe.GroupName,
		probe.IntervalSeconds, probe.TimeoutMs, probe.FailThreshold, probe.RecoverThreshold,
		probe.Status, probe.ConsecutiveFailures, probe.ConsecutiveSuccesses,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create probe"})
		return
	}

	if probe.Enabled && h.scheduler != nil {
		h.scheduler.StartProbe(probe)
	}

	c.JSON(http.StatusCreated, gin.H{"probe": probe})
}

func (h *Handler) UpdateProbe(c *gin.Context) {
	projectID := c.Param("projectId")
	probeID := c.Param("probeId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var old models.ProbeConfig
	err = h.db.Get(&old, "SELECT * FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Probe not found"})
		return
	}

	var req struct {
		Enabled          *bool   `json:"enabled"`
		GroupName        *string `json:"groupName"`
		IntervalSeconds  int     `json:"intervalSeconds"`
		TimeoutMs        int     `json:"timeoutMs"`
		FailThreshold    int     `json:"failThreshold"`
		RecoverThreshold int     `json:"recoverThreshold"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := old.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	groupName := old.GroupName
	if req.GroupName != nil {
		groupName = *req.GroupName
	}

	if enabled && !old.Enabled {
		var enabledCount int
		h.db.Get(&enabledCount,
			"SELECT COUNT(*) FROM probe_configs WHERE project_id = $1 AND enabled = true AND id != $2",
			projectID, probeID,
		)
		if enabledCount >= 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 20 enabled probes per project"})
			return
		}
	}

	intervalSeconds := req.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = old.IntervalSeconds
	}
	if intervalSeconds < 10 {
		intervalSeconds = 10
	}
	if intervalSeconds > 300 {
		intervalSeconds = 300
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = old.TimeoutMs
	}

	failThreshold := req.FailThreshold
	if failThreshold <= 0 {
		failThreshold = old.FailThreshold
	}

	recoverThreshold := req.RecoverThreshold
	if recoverThreshold <= 0 {
		recoverThreshold = old.RecoverThreshold
	}

	_, err = h.db.Exec(
		`UPDATE probe_configs SET enabled = $1, group_name = $2, interval_seconds = $3, timeout_ms = $4, fail_threshold = $5, recover_threshold = $6, updated_at = NOW() WHERE id = $7`,
		enabled, groupName, intervalSeconds, timeoutMs, failThreshold, recoverThreshold, probeID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update probe"})
		return
	}

	var updated models.ProbeConfig
	h.db.Get(&updated, "SELECT * FROM probe_configs WHERE id = $1", probeID)

	if h.scheduler != nil {
		if old.Enabled && !enabled {
			h.scheduler.StopProbe(probeID)
		} else if !old.Enabled && enabled {
			h.scheduler.StartProbe(updated)
		} else if enabled {
			h.scheduler.RestartProbe(updated)
		}
	}

	c.JSON(http.StatusOK, gin.H{"probe": updated})
}

func (h *Handler) DeleteProbe(c *gin.Context) {
	projectID := c.Param("projectId")
	probeID := c.Param("probeId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	_, err = h.db.Exec("DELETE FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete probe"})
		return
	}

	if h.scheduler != nil {
		h.scheduler.RemoveProbe(probeID)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Probe deleted"})
}

func (h *Handler) GetProbeDashboard(c *gin.Context) {
	projectID := c.Param("projectId")
	groupName := c.Query("groupName")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	type ProbeWithAPI struct {
		models.ProbeConfig
		APIPath       string `db:"api_path" json:"apiPath"`
		APIMethod     string `db:"api_method" json:"apiMethod"`
		APIDescription string `db:"api_description" json:"apiDescription"`
	}

	var probes []ProbeWithAPI
	var err error

	queryBase := `
		SELECT pc.*, a.path as api_path, a.method as api_method, a.description as api_description
		FROM probe_configs pc
		JOIN apis a ON pc.api_id = a.id
		WHERE pc.project_id = $1
	`
	if groupName != "" {
		err = h.db.Select(&probes, queryBase+` AND pc.group_name = $2 ORDER BY pc.created_at ASC`,
			projectID, groupName,
		)
	} else {
		err = h.db.Select(&probes, queryBase+` ORDER BY pc.created_at ASC`,
			projectID,
		)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load probes"})
		return
	}

	var groups []string
	h.db.Select(&groups, `
		SELECT DISTINCT group_name FROM probe_configs
		WHERE project_id = $1 AND group_name != ''
		ORDER BY group_name ASC
	`, projectID)

	healthyCount := 0
	degradedCount := 0
	unhealthyCount := 0

	type ProbeSummary struct {
		ID              string  `json:"id"`
		APIPath         string  `json:"apiPath"`
		APIMethod       string  `json:"apiMethod"`
		APIDescription  string  `json:"apiDescription"`
		GroupName       string  `json:"groupName"`
		Status          string  `json:"status"`
		Enabled         bool    `json:"enabled"`
		LastCheckTime   string  `json:"lastCheckTime"`
		LastResponseMs  int     `json:"lastResponseMs"`
		AvgResponseMs   int     `json:"avgResponseMs"`
		SuccessRate     float64 `json:"successRate"`
	}

	summaries := make([]ProbeSummary, 0, len(probes))

	for _, p := range probes {
		if !p.Enabled {
			continue
		}

		switch p.Status {
		case "healthy":
			healthyCount++
		case "degraded":
			degradedCount++
		case "unhealthy":
			unhealthyCount++
		}

		var lastRecord models.ProbeRecord
		err := h.db.Get(&lastRecord,
			"SELECT * FROM probe_records WHERE probe_id = $1 ORDER BY checked_at DESC LIMIT 1",
			p.ID,
		)

		lastCheckTime := ""
		lastResponseMs := 0
		if err == nil {
			lastCheckTime = lastRecord.CheckedAt.Format("2006-01-02T15:04:05Z07:00")
			lastResponseMs = lastRecord.ResponseTimeMs
		}

		var avgResult struct {
			AvgMs      *float64 `db:"avg_ms"`
			SuccessCnt int      `db:"success_cnt"`
			TotalCnt   int      `db:"total_cnt"`
		}
		h.db.Get(&avgResult, `
			SELECT AVG(response_time_ms) as avg_ms,
			       COUNT(*) FILTER (WHERE is_success = true) as success_cnt,
			       COUNT(*) as total_cnt
			FROM (
				SELECT * FROM probe_records
				WHERE probe_id = $1
				ORDER BY checked_at DESC
				LIMIT 50
			) sub
		`, p.ID)

		avgResponseMs := 0
		if avgResult.AvgMs != nil {
			avgResponseMs = int(*avgResult.AvgMs)
		}

		successRate := 0.0
		if avgResult.TotalCnt > 0 {
			successRate = float64(avgResult.SuccessCnt) / float64(avgResult.TotalCnt) * 100
		}

		summaries = append(summaries, ProbeSummary{
			ID:             p.ID,
			APIPath:        p.APIPath,
			APIMethod:      p.APIMethod,
			APIDescription: p.APIDescription,
			GroupName:      p.GroupName,
			Status:         p.Status,
			Enabled:        p.Enabled,
			LastCheckTime:  lastCheckTime,
			LastResponseMs: lastResponseMs,
			AvgResponseMs:  avgResponseMs,
			SuccessRate:    successRate,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": gin.H{
			"healthy":   healthyCount,
			"degraded":  degradedCount,
			"unhealthy": unhealthyCount,
		},
		"groups": groups,
		"probes": summaries,
	})
}

func (h *Handler) GetProbeDetail(c *gin.Context) {
	projectID := c.Param("projectId")
	probeID := c.Param("probeId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var probe models.ProbeConfig
	err := h.db.Get(&probe, "SELECT * FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Probe not found"})
		return
	}

	var records []models.ProbeRecord
	h.db.Select(&records, `
		SELECT * FROM probe_records
		WHERE probe_id = $1
		ORDER BY checked_at DESC
		LIMIT 50
	`, probeID)

	var alerts []models.AlertEvent
	h.db.Select(&alerts, `
		SELECT * FROM alert_events
		WHERE probe_id = $1
		ORDER BY triggered_at DESC
		LIMIT 50
	`, probeID)

	type RecordWithMeta struct {
		models.ProbeRecord
	}
	type AlertWithMeta struct {
		models.AlertEvent
	}

	recordsOut := make([]RecordWithMeta, 0, len(records))
	for _, r := range records {
		recordsOut = append(recordsOut, RecordWithMeta{ProbeRecord: r})
	}

	alertsOut := make([]AlertWithMeta, 0, len(alerts))
	for _, a := range alerts {
		alertsOut = append(alertsOut, AlertWithMeta{AlertEvent: a})
	}

	c.JSON(http.StatusOK, gin.H{
		"probe":   probe,
		"records": records,
		"alerts":  alerts,
	})
}

func (h *Handler) GetProbeAvailabilityTrend(c *gin.Context) {
	projectID := c.Param("projectId")
	probeID := c.Param("probeId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var probe models.ProbeConfig
	err := h.db.Get(&probe, "SELECT * FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Probe not found"})
		return
	}

	type HourlyStat struct {
		Hour       string  `db:"hour" json:"hour"`
		TotalCnt   int     `db:"total_cnt" json:"totalCnt"`
		SuccessCnt int     `db:"success_cnt" json:"successCnt"`
		SuccessRate float64 `db:"success_rate" json:"successRate"`
	}

	var stats []HourlyStat
	err = h.db.Select(&stats, `
		SELECT
			to_char(date_trunc('hour', checked_at), 'YYYY-MM-DD HH24:00:00') as hour,
			COUNT(*) as total_cnt,
			COUNT(*) FILTER (WHERE is_success = true) as success_cnt,
			ROUND(
				COUNT(*) FILTER (WHERE is_success = true)::numeric / NULLIF(COUNT(*), 0)::numeric * 100,
				2
			) as success_rate
		FROM probe_records
		WHERE probe_id = $1
		  AND checked_at >= NOW() - INTERVAL '24 hours'
		GROUP BY date_trunc('hour', checked_at)
		ORDER BY hour ASC
	`, probeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load availability trend"})
		return
	}

	type FullHour struct {
		Hour        string  `json:"hour"`
		TotalCnt    int     `json:"totalCnt"`
		SuccessCnt  int     `json:"successCnt"`
		SuccessRate float64 `json:"successRate"`
		HasData     bool    `json:"hasData"`
	}

	fullStats := make([]FullHour, 24)
	now := time.Now()
	for i := 23; i >= 0; i-- {
		hourTime := now.Add(time.Duration(-i) * time.Hour).Truncate(time.Hour)
		hourStr := hourTime.Format("2006-01-02 15:00:00")
		fullStats[23-i] = FullHour{
			Hour:    hourStr,
			HasData: false,
		}
	}

	statMap := make(map[string]HourlyStat)
	for _, s := range stats {
		statMap[s.Hour] = s
	}

	totalAll := 0
	successAll := 0

	for i := range fullStats {
		if stat, ok := statMap[fullStats[i].Hour]; ok {
			fullStats[i].TotalCnt = stat.TotalCnt
			fullStats[i].SuccessCnt = stat.SuccessCnt
			fullStats[i].SuccessRate = stat.SuccessRate
			fullStats[i].HasData = true
			totalAll += stat.TotalCnt
			successAll += stat.SuccessCnt
		}
	}

	overallRate := 0.0
	if totalAll > 0 {
		overallRate = float64(successAll) / float64(totalAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"hours":        fullStats,
		"overallRate":  overallRate,
		"totalCount":   totalAll,
		"successCount": successAll,
	})
}

func (h *Handler) GetProbeAlerts(c *gin.Context) {
	projectID := c.Param("projectId")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var alerts []models.AlertEvent
	err := h.db.Select(&alerts, `
		SELECT ae.* FROM alert_events ae
		JOIN probe_configs pc ON ae.probe_id = pc.id
		WHERE pc.project_id = $1
		ORDER BY ae.triggered_at DESC
		LIMIT 100
	`, projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load alerts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (h *Handler) CreateProbeForAPI(c *gin.Context) {
	projectID := c.Param("projectId")
	apiID := c.Param("id")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var api models.API
	err = h.db.Get(&api, "SELECT * FROM apis WHERE id = $1 AND project_id = $2", apiID, projectID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API not found in this project"})
		return
	}

	var existing int
	h.db.Get(&existing,
		"SELECT COUNT(*) FROM probe_configs WHERE api_id = $1",
		apiID,
	)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Probe already exists for this API"})
		return
	}

	var req struct {
		Enabled          *bool   `json:"enabled"`
		GroupName        string  `json:"groupName"`
		IntervalSeconds  int     `json:"intervalSeconds"`
		TimeoutMs        int     `json:"timeoutMs"`
		FailThreshold    int     `json:"failThreshold"`
		RecoverThreshold int     `json:"recoverThreshold"`
	}
	c.ShouldBindJSON(&req)

	enabled := false
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	if enabled {
		var enabledCount int
		h.db.Get(&enabledCount,
			"SELECT COUNT(*) FROM probe_configs WHERE project_id = $1 AND enabled = true",
			projectID,
		)
		if enabledCount >= 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Maximum 20 enabled probes per project"})
			return
		}
	}

	intervalSeconds := req.IntervalSeconds
	if intervalSeconds < 10 {
		intervalSeconds = 30
	}
	if intervalSeconds > 300 {
		intervalSeconds = 300
	}

	timeoutMs := req.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 3000
	}

	failThreshold := req.FailThreshold
	if failThreshold <= 0 {
		failThreshold = 3
	}

	recoverThreshold := req.RecoverThreshold
	if recoverThreshold <= 0 {
		recoverThreshold = 2
	}

	probe := models.ProbeConfig{
		ID:                   uuid.New().String(),
		APIID:                apiID,
		ProjectID:            projectID,
		Enabled:              enabled,
		GroupName:            req.GroupName,
		IntervalSeconds:      intervalSeconds,
		TimeoutMs:            timeoutMs,
		FailThreshold:        failThreshold,
		RecoverThreshold:     recoverThreshold,
		Status:               "healthy",
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 0,
	}

	_, err = h.db.Exec(
		`INSERT INTO probe_configs (id, api_id, project_id, enabled, group_name, interval_seconds, timeout_ms, fail_threshold, recover_threshold, status, consecutive_failures, consecutive_successes)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		probe.ID, probe.APIID, probe.ProjectID, probe.Enabled, probe.GroupName,
		probe.IntervalSeconds, probe.TimeoutMs, probe.FailThreshold, probe.RecoverThreshold,
		probe.Status, probe.ConsecutiveFailures, probe.ConsecutiveSuccesses,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create probe"})
		return
	}

	if probe.Enabled && h.scheduler != nil {
		h.scheduler.StartProbe(probe)
	}

	c.JSON(http.StatusCreated, gin.H{"probe": probe})
}

func (h *Handler) GetAPIProbe(c *gin.Context) {
	projectID := c.Param("projectId")
	apiID := c.Param("id")

	if !h.canAccessProject(c, projectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "No access to project"})
		return
	}

	var probe models.ProbeConfig
	err := h.db.Get(&probe, "SELECT * FROM probe_configs WHERE api_id = $1 AND project_id = $2", apiID, projectID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"probe": nil})
		return
	}

	c.JSON(http.StatusOK, gin.H{"probe": probe})
}

func (h *Handler) BatchEnableProbes(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		ProbeIDs []string `json:"probeIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success := 0
	skipped := 0
	failed := 0
	var skippedIDs []string

	var enabledCount int
	h.db.Get(&enabledCount,
		"SELECT COUNT(*) FROM probe_configs WHERE project_id = $1 AND enabled = true",
		projectID,
	)

	for _, probeID := range req.ProbeIDs {
		var probe models.ProbeConfig
		err := h.db.Get(&probe, "SELECT * FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
		if err != nil {
			failed++
			continue
		}

		if probe.Enabled {
			skipped++
			skippedIDs = append(skippedIDs, probeID)
			continue
		}

		if enabledCount >= 20 {
			skipped++
			skippedIDs = append(skippedIDs, probeID)
			continue
		}

		_, err = h.db.Exec("UPDATE probe_configs SET enabled = true, updated_at = NOW() WHERE id = $1", probeID)
		if err != nil {
			failed++
			continue
		}

		enabledCount++
		success++

		if h.scheduler != nil {
			var updated models.ProbeConfig
			h.db.Get(&updated, "SELECT * FROM probe_configs WHERE id = $1", probeID)
			h.scheduler.StartProbe(updated)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"skipped": skipped,
		"failed":  failed,
		"skippedIds": skippedIDs,
	})
}

func (h *Handler) BatchDisableProbes(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		ProbeIDs []string `json:"probeIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success := 0
	skipped := 0
	failed := 0

	for _, probeID := range req.ProbeIDs {
		var probe models.ProbeConfig
		err := h.db.Get(&probe, "SELECT * FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
		if err != nil {
			failed++
			continue
		}

		if !probe.Enabled {
			skipped++
			continue
		}

		_, err = h.db.Exec("UPDATE probe_configs SET enabled = false, updated_at = NOW() WHERE id = $1", probeID)
		if err != nil {
			failed++
			continue
		}

		success++

		if h.scheduler != nil {
			h.scheduler.StopProbe(probeID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"skipped": skipped,
		"failed":  failed,
	})
}

func (h *Handler) BatchDeleteProbes(c *gin.Context) {
	projectID := c.Param("projectId")

	role, err := h.getProjectRole(c, projectID)
	if err != nil || role == "viewer" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Editor or admin access required"})
		return
	}

	var req struct {
		ProbeIDs []string `json:"probeIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	success := 0
	failed := 0

	for _, probeID := range req.ProbeIDs {
		_, err := h.db.Exec("DELETE FROM probe_configs WHERE id = $1 AND project_id = $2", probeID, projectID)
		if err != nil {
			failed++
			continue
		}

		success++

		if h.scheduler != nil {
			h.scheduler.RemoveProbe(probeID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"skipped": 0,
		"failed":  failed,
	})
}

func (h *Handler) ProbeWebSocket(c *gin.Context) {
	projectID := c.Param("projectId")
	log.Printf("[WebSocket] Handshake request for project %s, RemoteAddr=%s", projectID, c.Request.RemoteAddr)

	tokenStr := ""
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		log.Printf("[WebSocket] Token from Authorization header, length=%d", len(tokenStr))
	}
	if tokenStr == "" {
		tokenStr = c.Query("token")
		log.Printf("[WebSocket] Token from query string, length=%d", len(tokenStr))
	}

	if tokenStr == "" {
		log.Printf("[WebSocket] ERROR: No token provided")
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("Token required"))
		return
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("[WebSocket] ERROR: Wrong signing method: %v", token.Method)
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		log.Printf("[WebSocket] ERROR: Invalid token, err=%v", err)
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("Invalid token"))
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("[WebSocket] ERROR: Invalid token claims type")
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("Invalid token claims"))
		return
	}

	var userID string
	switch v := claims["sub"].(type) {
	case string:
		userID = v
	case float64:
		userID = fmt.Sprintf("%.0f", v)
	default:
		log.Printf("[WebSocket] ERROR: Unexpected sub type %T", v)
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		c.Writer.Write([]byte("Invalid user id in token"))
		return
	}
	c.Set("userID", userID)
	log.Printf("[WebSocket] Authenticated userID=%s", userID)

	if !h.canAccessProject(c, projectID) {
		log.Printf("[WebSocket] ERROR: User %s has no access to project %s", userID, projectID)
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.WriteHeader(http.StatusForbidden)
		c.Writer.Write([]byte("No access to project"))
		return
	}

	log.Printf("[WebSocket] Upgrading to WebSocket...")
	ws.ServeWS(h.wsHub, projectID, c)
}
