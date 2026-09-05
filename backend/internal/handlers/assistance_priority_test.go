package handlers

import (
	"testing"

	"go-sse-server/internal/database"
)

func TestCalculateAssistancePriorityByCategory(t *testing.T) {
	tests := []struct {
		name     string
		category string
		want     database.RequestPriority
	}{
		{name: "medical is critical", category: "MEDICINE", want: database.RequestPriorityCRITICAL},
		{name: "water is high", category: "WATER", want: database.RequestPriorityHIGH},
		{name: "shelter is high", category: "SHELTER", want: database.RequestPriorityHIGH},
		{name: "food is medium", category: "FOOD", want: database.RequestPriorityMEDIUM},
		{name: "equipment is low", category: "EQUIPMENT", want: database.RequestPriorityLOW},
		{name: "volunteer is low", category: "VOLUNTEER", want: database.RequestPriorityLOW},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := calculateAssistancePriority(test.category, "", "", false); got != test.want {
				t.Fatalf("calculateAssistancePriority() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCalculateAssistancePriorityEscalatesVulnerability(t *testing.T) {
	if got := calculateAssistancePriority("FOOD", "food", "Need food for pregnant women and an infant", false); got != database.RequestPriorityCRITICAL {
		t.Fatalf("vulnerable food request = %q, want CRITICAL", got)
	}

	if got := calculateAssistancePriority("WATER", "water", "Elderly people need drinking water", false); got != database.RequestPriorityCRITICAL {
		t.Fatalf("vulnerable water request = %q, want CRITICAL", got)
	}
}

func TestCalculateAssistancePriorityEscalatesNearbyHazard(t *testing.T) {
	if got := calculateAssistancePriority("FOOD", "food", "Family needs meals", true); got != database.RequestPriorityHIGH {
		t.Fatalf("food near hazard = %q, want HIGH", got)
	}

	if got := calculateAssistancePriority("SHELTER", "shelter", "Roof collapsed", true); got != database.RequestPriorityCRITICAL {
		t.Fatalf("shelter near hazard = %q, want CRITICAL", got)
	}
}
