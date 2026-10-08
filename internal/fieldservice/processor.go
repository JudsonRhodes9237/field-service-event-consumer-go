package fieldservice

import "fmt"

type Processor struct {
	store *WorkOrderStore
}

func NewProcessor(store *WorkOrderStore) *Processor {
	return &Processor{store: store}
}

func (p *Processor) Process(evt PlatformEvent) WorkOrderState {
	current, _ := p.store.Get(evt.WorkOrder.ID)
	current.ID = evt.WorkOrder.ID
	current.TechnicianID = evt.Technician.ID

	if evt.Dispatch != nil && evt.Dispatch.Status != "" {
		current.DispatchStatus = evt.Dispatch.Status
	}

	if evt.EventType == "photo_uploaded" && evt.Photo != nil {
		current.PhotoCount++
		if current.DispatchStatus == "pending" || current.DispatchStatus == "" {
			current.NeedsFollowUp = true
			current.LastNote = "Photo received before dispatch confirmation; contact technician."
		} else {
			current.LastNote = fmt.Sprintf("Photo %s received.", evt.Photo.PhotoID)
		}
	}

	if evt.EventType == "dispatch_updated" && evt.Dispatch != nil {
		if evt.Dispatch.Status == "completed" {
			current.NeedsFollowUp = false
			current.LastNote = "Dispatch completed."
		}
	}

	p.store.Apply(current)
	return current
}
