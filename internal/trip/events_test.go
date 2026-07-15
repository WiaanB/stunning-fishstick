package trip

import (
	"testing"

	"github.com/google/uuid"
)

// TestEventTypesAndAggregateID locks in each domain event's wire EventType
// and confirms AggregateID round-trips the trip ID it was constructed with.
// In particular it pins TripStarted to "trip.started" (matching the struct
// name) rather than the old "trip.in_progress" mismatch.
func TestEventTypesAndAggregateID(t *testing.T) {
	tripID := uuid.New()
	cases := []struct {
		name      string
		event     Event
		eventType string
	}{
		{"TripRequested", TripRequested{TripID: tripID}, "trip.requested"},
		{"TripQuoted", TripQuoted{TripID: tripID}, "trip.quoted"},
		{"TripPaymentAwaited", TripPaymentAwaited{TripID: tripID}, "trip.payment_awaited"},
		{"TripCodeIssued", TripCodeIssued{TripID: tripID}, "trip.code_issued"},
		{"TripDriverAssigned", TripDriverAssigned{TripID: tripID}, "trip.driver_assigned"},
		{"TripEnRoute", TripEnRoute{TripID: tripID}, "trip.en_route"},
		{"TripCodeVerified", TripCodeVerified{TripID: tripID}, "trip.code_verified"},
		{"TripStarted", TripStarted{TripID: tripID}, "trip.started"},
		{"TripCompleted", TripCompleted{TripID: tripID}, "trip.completed"},
		{"TripCancelled", TripCancelled{TripID: tripID}, "trip.cancelled"},
		{"TripNoShow", TripNoShow{TripID: tripID}, "trip.no_show"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.event.EventType(); got != c.eventType {
				t.Fatalf("EventType() = %q, want %q", got, c.eventType)
			}
			if got := c.event.AggregateID(); got != tripID {
				t.Fatalf("AggregateID() = %s, want %s", got, tripID)
			}
		})
	}
}
