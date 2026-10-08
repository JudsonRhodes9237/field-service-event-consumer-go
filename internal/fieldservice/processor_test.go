package fieldservice

import "testing"

func TestProcessorProcessFollowUpDecision(t *testing.T) {
	store := NewWorkOrderStore()
	p := NewProcessor(store)

	tests := []struct {
		name     string
		event    PlatformEvent
		wantNote string
		wantFlag bool
		wantDisp string
	}{
		{
			name: "photo before dispatch confirmation triggers follow up",
			event: PlatformEvent{
				EventID:   "evt-1",
				EventType: "photo_uploaded",
				WorkOrder: WorkOrderRef{ID: "WO-42", Customer: "North Clinic"},
				Technician: TechnicianRef{ID: "tech-7", Name: "Riley"},
				Photo:     &PhotoPayload{PhotoID: "photo-9", Category: "before"},
				Dispatch:  &DispatchUpdate{Status: "pending"},
			},
			wantNote: "Photo received before dispatch confirmation; contact technician.",
			wantFlag: true,
			wantDisp: "pending",
		},
		{
			name: "completed dispatch clears follow up",
			event: PlatformEvent{
				EventID:   "evt-2",
				EventType: "dispatch_updated",
				WorkOrder: WorkOrderRef{ID: "WO-42", Customer: "North Clinic"},
				Technician: TechnicianRef{ID: "tech-7", Name: "Riley"},
				Dispatch:  &DispatchUpdate{Status: "completed"},
			},
			wantNote: "Dispatch completed.",
			wantFlag: false,
			wantDisp: "completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Process(tt.event)
			if got.LastNote != tt.wantNote {
				t.Fatalf("LastNote = %q, want %q", got.LastNote, tt.wantNote)
			}
			if got.NeedsFollowUp != tt.wantFlag {
				t.Fatalf("NeedsFollowUp = %v, want %v", got.NeedsFollowUp, tt.wantFlag)
			}
			if got.DispatchStatus != tt.wantDisp {
				t.Fatalf("DispatchStatus = %q, want %q", got.DispatchStatus, tt.wantDisp)
			}
		})
	}
}
