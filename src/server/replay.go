package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"time"
)

const defaultPageSize = 1000
const defaultRetention = 24 * time.Hour

// ReplayQuery defines filters for querying kernel events.
type ReplayQuery struct {
	NodeName     string
	From         time.Time
	To           time.Time
	EventTypes   []string // "syscall", "process", "network"
	PodName      string
	MinLatencyNs int64
	Page         int
	PageSize     int
}

// ReplayResponse is the paginated response for replay queries.
type ReplayResponse struct {
	Events     []EnrichedEvent `json:"events"`
	TotalCount int             `json:"totalCount"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
}

// ReplayStore persists and queries kernel events for post-mortem replay.
type ReplayStore struct {
	store     Store
	retention time.Duration
}

// NewReplayStore creates a new replay store with the given retention period.
func NewReplayStore(store Store, retention time.Duration) *ReplayStore {
	if retention <= 0 {
		retention = defaultRetention
	}
	return &ReplayStore{store: store, retention: retention}
}

// Query returns paginated kernel events matching the filter.
func (rs *ReplayStore) Query(ctx context.Context, q ReplayQuery) ([]EnrichedEvent, int, error) {
	if q.PageSize <= 0 {
		q.PageSize = defaultPageSize
	}
	if q.Page < 1 {
		q.Page = 1
	}

	// Enforce retention window
	earliest := time.Now().Add(-rs.retention)
	if q.From.Before(earliest) {
		q.From = earliest
	}

	var events []EnrichedEvent
	var err error

	if len(q.EventTypes) == 1 {
		events, err = rs.store.GetKernelEventsByType(ctx, q.NodeName, q.EventTypes[0], q.From, q.To)
	} else {
		events, err = rs.store.GetKernelEvents(ctx, q.NodeName, q.From, q.To)
	}
	if err != nil {
		return nil, 0, err
	}

	// Apply additional filters
	filtered := filterEvents(events, q)

	// Sort chronologically
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	totalCount := len(filtered)

	// Paginate
	start := (q.Page - 1) * q.PageSize
	if start >= totalCount {
		return []EnrichedEvent{}, totalCount, nil
	}
	end := start + q.PageSize
	if end > totalCount {
		end = totalCount
	}

	return filtered[start:end], totalCount, nil
}

// filterEvents applies all query filters to the event list.
func filterEvents(events []EnrichedEvent, q ReplayQuery) []EnrichedEvent {
	var result []EnrichedEvent
	typeSet := make(map[string]bool)
	for _, t := range q.EventTypes {
		typeSet[t] = true
	}

	for _, e := range events {
		// Filter by event type (if multiple types specified)
		if len(typeSet) > 1 {
			if !typeSet[e.EventType] {
				continue
			}
		}

		// Filter by pod name
		if q.PodName != "" && e.PodName != q.PodName {
			continue
		}

		// Filter by minimum latency
		if q.MinLatencyNs > 0 && int64(e.LatencyNs) < q.MinLatencyNs {
			continue
		}

		result = append(result, e)
	}
	return result
}

// replayHandler serves GET /api/replay with query parameters.
func replayHandler(rs *ReplayStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		q := ReplayQuery{
			NodeName: r.URL.Query().Get("node"),
			PodName:  r.URL.Query().Get("pod"),
		}

		if q.NodeName == "" {
			writeJSONError(w, "node parameter is required", http.StatusBadRequest)
			return
		}

		// Parse time range
		if fromStr := r.URL.Query().Get("from"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				q.From = t
			}
		}
		if q.From.IsZero() {
			q.From = time.Now().Add(-1 * time.Hour)
		}

		if toStr := r.URL.Query().Get("to"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				q.To = t
			}
		}
		if q.To.IsZero() {
			q.To = time.Now()
		}

		// Parse event types
		if typeStr := r.URL.Query().Get("type"); typeStr != "" {
			q.EventTypes = []string{typeStr}
		}

		// Parse min latency
		if latStr := r.URL.Query().Get("minLatency"); latStr != "" {
			if v, err := strconv.ParseInt(latStr, 10, 64); err == nil {
				q.MinLatencyNs = v
			}
		}

		// Parse pagination
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if v, err := strconv.Atoi(pageStr); err == nil {
				q.Page = v
			}
		}
		if psStr := r.URL.Query().Get("pageSize"); psStr != "" {
			if v, err := strconv.Atoi(psStr); err == nil {
				q.PageSize = v
			}
		}

		events, totalCount, err := rs.Query(r.Context(), q)
		if err != nil {
			writeJSONError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if events == nil {
			events = []EnrichedEvent{}
		}

		resp := ReplayResponse{
			Events:     events,
			TotalCount: totalCount,
			Page:       q.Page,
			PageSize:   q.PageSize,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
