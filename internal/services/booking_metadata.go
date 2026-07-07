package services

import (
	"encoding/json"

	"lodge-system/internal/models"

	"github.com/google/uuid"
)

func buildRoomMetadata(roomID uuid.UUID, roomName, roomType, checkIn, checkOut string, nights int) json.RawMessage {
	m := map[string]interface{}{
		"room_id":   roomID,
		"room_name": roomName,
		"room_type": roomType,
		"check_in":  checkIn,
		"check_out": checkOut,
		"nights":    nights,
	}
	b, _ := json.Marshal(m)
	return b
}

func buildEventMetadata(envelope *models.SubmitEventBookingRequest) json.RawMessage {
	if envelope.Event == nil {
		return nil
	}
	type sessionSummary struct {
		EventType string `json:"event_type,omitempty"`
		EventDate string `json:"event_date,omitempty"`
		StartTime string `json:"start_time,omitempty"`
		EndTime   string `json:"end_time,omitempty"`
		VenueName string `json:"venue_name,omitempty"`
		Pax       int    `json:"pax,omitempty"`
	}
	sessions := make([]sessionSummary, 0, len(envelope.Event.Sessions))
	for _, s := range envelope.Event.Sessions {
		sessions = append(sessions, sessionSummary{
			EventType: s.EventType,
			EventDate: s.EventDate,
			StartTime: s.StartTime,
			EndTime:   s.EndTime,
			VenueName: s.VenueName,
			Pax:       s.ExpectedAttendees,
		})
	}
	m := map[string]interface{}{
		"start_date": envelope.Event.StartDate,
		"end_date":   envelope.Event.EndDate,
		"sessions":   sessions,
	}
	// Preserve the corporate billing blocks so the invoice can hydrate TPIN, GL
	// code, cost center, department and approver (see Invoice.HydrateFromMetadata).
	if envelope.Company != nil {
		m["company"] = envelope.Company
	}
	if envelope.Approver != nil {
		m["approver"] = envelope.Approver
	}
	b, _ := json.Marshal(m)
	return b
}

func buildMealMetadata(envelope *models.SubmitMealBookingRequest) json.RawMessage {
	if envelope.Meal == nil {
		return nil
	}
	headcount := 0
	if envelope.ParticipantCount != nil {
		headcount = *envelope.ParticipantCount
	}
	m := map[string]interface{}{
		"start_date": envelope.Meal.StartDate,
		"end_date":   envelope.Meal.EndDate,
		"headcount":  headcount,
	}
	// Preserve the corporate billing blocks so the invoice can hydrate TPIN, GL
	// code, cost center, department and approver (see Invoice.HydrateFromMetadata).
	if envelope.Company != nil {
		m["company"] = envelope.Company
	}
	if envelope.Approver != nil {
		m["approver"] = envelope.Approver
	}
	b, _ := json.Marshal(m)
	return b
}
