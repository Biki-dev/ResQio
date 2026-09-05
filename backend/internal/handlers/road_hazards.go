package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-sse-server/internal/database"
	"go-sse-server/internal/middleware"
	"go-sse-server/internal/ml"
)

type SubmitRoadHazardRequest struct {
	Name        string   `json:"name"`
	PhoneNumber string   `json:"phone_number"`
	HazardType  string   `json:"hazard_type,omitempty"`
	Severity    string   `json:"severity,omitempty"`
	Description string   `json:"description,omitempty"`
	PhotoURL    string   `json:"photo_url,omitempty"`
	Location    string   `json:"location,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

type RoadHazardResponse struct {
	ID              string    `json:"id"`
	ReporterID      *string   `json:"reporter_id,omitempty"`
	Name            string    `json:"name"`
	PhoneNumber     string    `json:"phone_number"`
	HazardType      string    `json:"hazard_type"`
	Severity        string    `json:"severity"`
	Location        string    `json:"location"`
	Description     string    `json:"description"`
	IsVerified      bool      `json:"is_verified"`
	ImageURL        string    `json:"image_url,omitempty"`
	PredictedClass  string    `json:"predicted_class,omitempty"`
	ConfidenceScore *float64  `json:"confidence_score,omitempty"`
	PriorityScore   float64   `json:"priority_score"`
	ClusterID       *string   `json:"cluster_id,omitempty"`
	ClusterSize     int32     `json:"cluster_size"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *APIHandler) SubmitRoadHazard(w http.ResponseWriter, r *http.Request) {
	var req SubmitRoadHazardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	req.HazardType = strings.TrimSpace(req.HazardType)
	req.Severity = strings.TrimSpace(req.Severity)
	req.Description = strings.TrimSpace(req.Description)

	if req.HazardType == "" {
		req.HazardType = "ROAD_INCIDENT"
	}
	if req.Severity == "" {
		req.Severity = "MEDIUM"
	}

	// Resolve Location
	location := strings.TrimSpace(req.Location)
	if location == "" && req.Latitude != nil && req.Longitude != nil {
		location = fmt.Sprintf("POINT(%f %f)", *req.Longitude, *req.Latitude)
	}
	if location == "" {
		respondWithError(w, http.StatusBadRequest, "location or latitude/longitude is required")
		return
	}

	var prediction ml.ClassificationResult
	if req.PhotoURL != "" {
		if result, err := h.mlClient.PredictImage(r.Context(), req.PhotoURL); err == nil {
			prediction = result
		}
	}
	if prediction.Class == "" {
		prediction.Class = req.HazardType
	}
	priorityScore := hazardPriority(req.Severity, prediction.Confidence)
	clusterID, clusterSize := h.findHazardCluster(r, prediction.Class, location)
	if !clusterID.Valid {
		clusterID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	}

	// Resolve optional reporter_id from JWT claims
	var reporterID pgtype.UUID
	if claims, ok := middleware.GetClaims(r.Context()); ok && claims.AccountID != "" {
		if parsed, err := uuid.Parse(claims.AccountID); err == nil {
			reporterID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}

	hazard, err := h.queries.CreateRoadHazard(r.Context(), database.CreateRoadHazardParams{
		ReporterID:      reporterID,
		ReporterName:    textToPgText(req.Name),
		ReporterPhone:   textToPgText(req.PhoneNumber),
		HazardType:      req.HazardType,
		Severity:        req.Severity,
		Location:        location,
		Description:     textToPgText(req.Description),
		IsVerified:      false,
		ImageUrl:        textToPgText(req.PhotoURL),
		PredictedClass:  textToPgText(prediction.Class),
		ConfidenceScore: pgtype.Float8{Float64: prediction.Confidence, Valid: prediction.Confidence > 0},
		PriorityScore:   priorityScore,
		ClusterID:       clusterID,
		ClusterSize:     clusterSize + 1,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to submit issue: %v", err))
		return
	}
	clusterSize++
	_, _ = h.pool.Exec(r.Context(), `UPDATE road_hazards SET cluster_size = $1 WHERE cluster_id = $2`, clusterSize, clusterID)
	hazard.ClusterSize = clusterSize

	var reporterIDStr *string
	if hazard.ReporterID.Valid {
		str := uuid.UUID(hazard.ReporterID.Bytes).String()
		reporterIDStr = &str
	}

	respondWithJSON(w, http.StatusCreated, roadHazardResponse(hazard, reporterIDStr))
}

func hazardPriority(severity string, confidence float64) float64 {
	base := map[string]float64{"LOW": 1, "MEDIUM": 2, "HIGH": 4, "CRITICAL": 6}[strings.ToUpper(severity)]
	return base + math.Max(0, math.Min(1, confidence))*2
}

func (h *APIHandler) findHazardCluster(r *http.Request, predictedClass, location string) (pgtype.UUID, int32) {
	if predictedClass == "" || location == "" {
		return pgtype.UUID{}, 0
	}
	var clusterID pgtype.UUID
	var clusterSize int32
	err := h.pool.QueryRow(r.Context(), `
		SELECT cluster_id, cluster_size
		FROM road_hazards
		WHERE predicted_class = $1
		  AND ST_DWithin(location::geography, ST_GeomFromText($2, 4326)::geography, 100)
		ORDER BY created_at DESC
		LIMIT 1`, predictedClass, location).Scan(&clusterID, &clusterSize)
	if err != nil {
		return pgtype.UUID{}, 0
	}
	return clusterID, clusterSize
}

func roadHazardResponse(hazard database.RoadHazard, reporterID *string) RoadHazardResponse {
	var clusterID *string
	if hazard.ClusterID.Valid {
		value := uuid.UUID(hazard.ClusterID.Bytes).String()
		clusterID = &value
	}
	var confidence *float64
	if hazard.ConfidenceScore.Valid {
		value := hazard.ConfidenceScore.Float64
		confidence = &value
	}
	return RoadHazardResponse{
		ID: uuid.UUID(hazard.ID.Bytes).String(), ReporterID: reporterID,
		Name: pgTextToString(hazard.ReporterName), PhoneNumber: pgTextToString(hazard.ReporterPhone),
		HazardType: hazard.HazardType, Severity: hazard.Severity, Location: hazard.Location,
		Description: pgTextToString(hazard.Description), IsVerified: hazard.IsVerified,
		ImageURL: pgTextToString(hazard.ImageUrl), PredictedClass: pgTextToString(hazard.PredictedClass),
		ConfidenceScore: confidence, PriorityScore: hazard.PriorityScore, ClusterID: clusterID,
		ClusterSize: hazard.ClusterSize, CreatedAt: hazard.CreatedAt.Time,
	}
}

func (h *APIHandler) ListRoadHazards(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	offset := int32(0)

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	var hazards []database.RoadHazard
	var err error

	reporterIDStr := r.URL.Query().Get("reporter_id")
	if reporterIDStr != "" {
		parsed, parseErr := uuid.Parse(reporterIDStr)
		if parseErr != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid reporter_id format")
			return
		}
		hazards, err = h.queries.ListRoadHazardsByReporter(r.Context(), database.ListRoadHazardsByReporterParams{
			ReporterID: pgtype.UUID{Bytes: parsed, Valid: true},
			Limit:      limit,
			Offset:     offset,
		})
	} else {
		hazards, err = h.queries.ListRoadHazards(r.Context(), database.ListRoadHazardsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve issue submissions")
		return
	}

	res := make([]RoadHazardResponse, 0, len(hazards))
	for _, hazard := range hazards {
		var reporterIDStr *string
		if hazard.ReporterID.Valid {
			str := uuid.UUID(hazard.ReporterID.Bytes).String()
			reporterIDStr = &str
		}

		res = append(res, roadHazardResponse(hazard, reporterIDStr))
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (h *APIHandler) GetRoadHazardByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid issue ID format")
		return
	}

	hazard, err := h.queries.GetRoadHazardByID(r.Context(), pgtype.UUID{Bytes: parsedID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Issue submission not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve issue submission")
		return
	}

	var reporterIDStr *string
	if hazard.ReporterID.Valid {
		str := uuid.UUID(hazard.ReporterID.Bytes).String()
		reporterIDStr = &str
	}

	respondWithJSON(w, http.StatusOK, roadHazardResponse(hazard, reporterIDStr))
}
