package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"api-mocker/models"
)

type Scheduler struct {
	db       *sqlx.DB
	mockURL  string
	mu       sync.RWMutex
	probes   map[string]*probeWorker
	stopCh   chan struct{}
	cleanCh  chan struct{}
}

type probeWorker struct {
	config   models.ProbeConfig
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
}

func NewScheduler(db *sqlx.DB, mockURL string) *Scheduler {
	return &Scheduler{
		db:      db,
		mockURL: mockURL,
		probes:  make(map[string]*probeWorker),
		stopCh:  make(chan struct{}),
		cleanCh: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	log.Println("[probe] Scheduler starting, loading enabled probes...")
	var configs []models.ProbeConfig
	err := s.db.Select(&configs,
		"SELECT * FROM probe_configs WHERE enabled = true",
	)
	if err != nil {
		log.Printf("[probe] Error loading probe configs: %v", err)
		return
	}
	for _, cfg := range configs {
		s.startProbe(cfg)
	}
	go s.cleanupLoop()
	log.Printf("[probe] Scheduler started with %d active probes", len(configs))
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, w := range s.probes {
		close(w.stopCh)
		w.ticker.Stop()
		delete(s.probes, id)
	}
}

func (s *Scheduler) StartProbe(cfg models.ProbeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, exists := s.probes[cfg.ID]; exists {
		close(w.stopCh)
		w.ticker.Stop()
		delete(s.probes, cfg.ID)
	}
	s.startProbeUnlocked(cfg)
}

func (s *Scheduler) StopProbe(probeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, exists := s.probes[probeID]; exists {
		close(w.stopCh)
		w.ticker.Stop()
		delete(s.probes, probeID)
	}
}

func (s *Scheduler) RestartProbe(cfg models.ProbeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, exists := s.probes[cfg.ID]; exists {
		close(w.stopCh)
		w.ticker.Stop()
		delete(s.probes, cfg.ID)
	}
	if cfg.Enabled {
		s.startProbeUnlocked(cfg)
	}
}

func (s *Scheduler) RemoveProbe(probeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, exists := s.probes[probeID]; exists {
		close(w.stopCh)
		w.ticker.Stop()
		delete(s.probes, probeID)
	}
}

func (s *Scheduler) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.probes)
}

func (s *Scheduler) startProbe(cfg models.ProbeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startProbeUnlocked(cfg)
}

func (s *Scheduler) startProbeUnlocked(cfg models.ProbeConfig) {
	if !cfg.Enabled {
		return
	}
	w := &probeWorker{
		config:  cfg,
		ticker:  time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second),
		stopCh:  make(chan struct{}),
		running: true,
	}
	s.probes[cfg.ID] = w
	go s.runProbe(w)
}

func (s *Scheduler) runProbe(w *probeWorker) {
	go s.executeCheck(w.config)
	for {
		select {
		case <-w.ticker.C:
			s.executeCheck(w.config)
		case <-w.stopCh:
			return
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) executeCheck(cfg models.ProbeConfig) {
	var api models.API
	err := s.db.Get(&api, "SELECT * FROM apis WHERE id = $1", cfg.APIID)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/mock/%s%s", s.mockURL, cfg.ProjectID, api.Path)
	req, err := http.NewRequest(strings.ToUpper(api.Method), url, nil)
	if err != nil {
		s.recordResult(cfg, 0, 0, 0, false)
		return
	}

	s.applyDefaultParams(req, api)

	client := &http.Client{
		Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		s.recordResult(cfg, 0, int(elapsed), 0, false)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodySize := len(body)
	statusCode := resp.StatusCode
	isSuccess := statusCode >= 200 && statusCode <= 299 && elapsed <= int64(cfg.TimeoutMs)

	s.recordResult(cfg, statusCode, int(elapsed), bodySize, isSuccess)
}

func (s *Scheduler) applyDefaultParams(req *http.Request, api models.API) {
	var params []models.ParamField
	if len(api.Params) > 0 {
		json.Unmarshal([]byte(api.Params), &params)
	}

	for _, p := range params {
		if p.Example == "" {
			continue
		}
		switch p.In {
		case "query":
			q := req.URL.Query()
			q.Set(p.Name, p.Example)
			req.URL.RawQuery = q.Encode()
		case "header":
			req.Header.Set(p.Name, p.Example)
		}
	}

	if len(api.RequestBody) > 0 && string(api.RequestBody) != "{}" {
		var bodyMap map[string]interface{}
		if json.Unmarshal([]byte(api.RequestBody), &bodyMap) == nil {
			examples := extractExamples(bodyMap)
			if len(examples) > 0 {
				bodyBytes, _ := json.Marshal(examples)
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				req.ContentLength = int64(len(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
			}
		}
	}
}

func extractExamples(bodyMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range bodyMap {
		if m, ok := v.(map[string]interface{}); ok {
			if example, exists := m["example"]; exists {
				result[k] = example
			} else if children, exists := m["children"]; exists {
				if childArr, ok := children.([]interface{}); ok && len(childArr) > 0 {
					if childMap, ok := childArr[0].(map[string]interface{}); ok {
						result[k] = extractExamples(childMap)
					}
				}
			}
		}
	}
	return result
}

func (s *Scheduler) recordResult(cfg models.ProbeConfig, statusCode int, responseTimeMs int, responseSize int, isSuccess bool) {
	record := models.ProbeRecord{
		ID:             uuid.New().String(),
		ProbeID:        cfg.ID,
		StatusCode:     statusCode,
		ResponseTimeMs: responseTimeMs,
		ResponseSize:   responseSize,
		IsSuccess:      isSuccess,
		CheckedAt:      time.Now(),
	}

	_, err := s.db.Exec(
		`INSERT INTO probe_records (id, probe_id, status_code, response_time_ms, response_size, is_success, checked_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		record.ID, record.ProbeID, record.StatusCode, record.ResponseTimeMs,
		record.ResponseSize, record.IsSuccess, record.CheckedAt,
	)
	if err != nil {
		log.Printf("[probe] Error inserting probe record: %v", err)
		return
	}

	s.updateStateMachine(cfg, isSuccess, statusCode, responseTimeMs)
}

func (s *Scheduler) updateStateMachine(cfg models.ProbeConfig, isSuccess bool, statusCode int, responseTimeMs int) {
	oldStatus := cfg.Status
	var newConsecFail int
	var newConsecSuccess int
	var newStatus string

	if isSuccess {
		newConsecFail = 0
		newConsecSuccess = cfg.ConsecutiveSuccesses + 1
		if oldStatus == "unhealthy" && newConsecSuccess >= cfg.RecoverThreshold {
			newStatus = "healthy"
		} else {
			newStatus = oldStatus
		}
	} else {
		newConsecSuccess = 0
		newConsecFail = cfg.ConsecutiveFailures + 1
		if newConsecFail >= cfg.FailThreshold {
			newStatus = "unhealthy"
		} else if newConsecFail == 1 {
			newStatus = "degraded"
		} else {
			newStatus = oldStatus
		}
	}

	if newStatus == "" {
		newStatus = oldStatus
	}

	_, err := s.db.Exec(
		`UPDATE probe_configs SET status = $1, consecutive_failures = $2, consecutive_successes = $3, updated_at = NOW() WHERE id = $4`,
		newStatus, newConsecFail, newConsecSuccess, cfg.ID,
	)
	if err != nil {
		log.Printf("[probe] Error updating probe state: %v", err)
		return
	}

	if newStatus != oldStatus {
		s.createAlertEvent(cfg, oldStatus, newStatus, statusCode, responseTimeMs)

		s.mu.RLock()
		if w, exists := s.probes[cfg.ID]; exists && w.running {
			w.config.Status = newStatus
			w.config.ConsecutiveFailures = newConsecFail
			w.config.ConsecutiveSuccesses = newConsecSuccess
		}
		s.mu.RUnlock()
	}
}

func (s *Scheduler) createAlertEvent(cfg models.ProbeConfig, oldStatus, newStatus string, statusCode, responseTimeMs int) {
	var api models.API
	probeName := cfg.ID
	if err := s.db.Get(&api, "SELECT * FROM apis WHERE id = $1", cfg.APIID); err == nil {
		probeName = fmt.Sprintf("%s %s", api.Method, api.Path)
	}

	event := models.AlertEvent{
		ID:                 uuid.New().String(),
		ProbeID:            cfg.ID,
		ProbeName:          probeName,
		OldStatus:          oldStatus,
		NewStatus:          newStatus,
		LastResponseTimeMs: responseTimeMs,
		LastStatusCode:     statusCode,
		TriggeredAt:        time.Now(),
	}

	_, err := s.db.Exec(
		`INSERT INTO alert_events (id, probe_id, probe_name, old_status, new_status, last_response_time_ms, last_status_code, triggered_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		event.ID, event.ProbeID, event.ProbeName, event.OldStatus, event.NewStatus,
		event.LastResponseTimeMs, event.LastStatusCode, event.TriggeredAt,
	)
	if err != nil {
		log.Printf("[probe] Error creating alert event: %v", err)
	}
}

func (s *Scheduler) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) cleanup() {
	now := time.Now()

	_, err := s.db.Exec(
		`DELETE FROM probe_records WHERE checked_at < $1`,
		now.AddDate(0, 0, -7),
	)
	if err != nil {
		log.Printf("[probe] Error cleaning up probe records: %v", err)
	}

	_, err = s.db.Exec(
		`DELETE FROM alert_events WHERE triggered_at < $1`,
		now.AddDate(0, 0, -30),
	)
	if err != nil {
		log.Printf("[probe] Error cleaning up alert events: %v", err)
	}
}
