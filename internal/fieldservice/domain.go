package fieldservice

import (
	"encoding/json"
	"sync"
)

type PlatformEvent struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt string          `json:"occurred_at"`
	WorkOrder  WorkOrderRef    `json:"work_order"`
	Technician TechnicianRef   `json:"technician"`
	Photo      *PhotoPayload   `json:"photo,omitempty"`
	Dispatch   *DispatchUpdate `json:"dispatch,omitempty"`
}

type WorkOrderRef struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
}

type TechnicianRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PhotoPayload struct {
	PhotoID  string `json:"photo_id"`
	Category string `json:"category"`
}

type DispatchUpdate struct {
	Status string `json:"status"`
}

type WorkOrderState struct {
	ID             string `json:"id"`
	DispatchStatus string `json:"dispatch_status"`
	PhotoCount     int    `json:"photo_count"`
	NeedsFollowUp  bool   `json:"needs_follow_up"`
	LastNote       string `json:"last_note"`
	TechnicianID   string `json:"technician_id"`
}

type WorkOrderStore struct {
	mu    sync.RWMutex
	items map[string]WorkOrderState
}

func NewWorkOrderStore() *WorkOrderStore {
	return &WorkOrderStore{items: make(map[string]WorkOrderState)}
}

func (s *WorkOrderStore) Apply(next WorkOrderState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[next.ID] = next
}

func (s *WorkOrderStore) Get(id string) (WorkOrderState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[id]
	return v, ok
}

func DecodeEvent(data json.RawMessage) (PlatformEvent, error) {
	var evt PlatformEvent
	err := json.Unmarshal(data, &evt)
	return evt, err
}
