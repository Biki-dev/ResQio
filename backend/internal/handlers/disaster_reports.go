package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-sse-server/internal/database"
	"go-sse-server/internal/middleware"
)

type DisasterReportCreateRequest struct {
	ReporterID string    `json:"reporter_id,omitempty"`
	Embedding  []float32 `json:"embedding,omitempty"`
	VectorStr  string    `json:"vector_str,omitempty"`
	Location   string    `json:"location,omitempty"`
	Latitude   *float64  `json:"latitude,omitempty"`
	Longitude  *float64  `json:"longitude,omitempty"`
}

type DisasterReportResponse struct {
	ID         string    `json:"id"`
	ReporterID string    `json:"reporter_id"`
	Location   string    `json:"location"`
	CreatedAt  time.Time `json:"created_at"`
}

func (h *APIHandler) CreateDisasterReport(w http.ResponseWriter, r *http.Request) {
	var req DisasterReportCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// 1. Resolve Location
	location := strings.TrimSpace(req.Location)
	if location == "" && req.Latitude != nil && req.Longitude != nil {
		location = fmt.Sprintf("POINT(%f %f)", *req.Longitude, *req.Latitude)
	}
	if location == "" {
		respondWithError(w, http.StatusBadRequest, "location or latitude/longitude is required")
		return
	}

	// 2. Resolve Embedding string representation for pgvector
	var embeddingStr string
	if len(req.Embedding) > 0 {
		var parts []string
		for _, v := range req.Embedding {
			parts = append(parts, fmt.Sprintf("%f", v))
		}
		embeddingStr = "[" + strings.Join(parts, ",") + "]"
	} else if strings.TrimSpace(req.VectorStr) != "" {
		embeddingStr = strings.TrimSpace(req.VectorStr)
	} else {
		// Default 1536-dimensional zero vector if none passed
		embeddingStr = "[" + strings.Repeat("0,", 1535) + "0]"
	}

	// 3. Resolve Reporter ID (from claims, request, or random UUID)
	var reporterID pgtype.UUID
	if claims, ok := middleware.GetClaims(r.Context()); ok && claims.AccountID != "" {
		if parsed, err := uuid.Parse(claims.AccountID); err == nil {
			reporterID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}
	if !reporterID.Valid && req.ReporterID != "" {
		if parsed, err := uuid.Parse(req.ReporterID); err == nil {
			reporterID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}
	if !reporterID.Valid {
		reporterID = pgtype.UUID{Bytes: uuid.New(), Valid: true}
	}

	report, err := h.queries.CreateDisasterReport(r.Context(), database.CreateDisasterReportParams{
		ReporterID: reporterID,
		Embedding:  embeddingStr,
		Location:   location,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to store disaster report: %v", err))
		return
	}

	respondWithJSON(w, http.StatusCreated, DisasterReportResponse{
		ID:         uuid.UUID(report.ID.Bytes).String(),
		ReporterID: uuid.UUID(report.ReporterID.Bytes).String(),
		Location:   report.Location,
		CreatedAt:  report.CreatedAt.Time,
	})
}

func (h *APIHandler) ListDisasterReports(w http.ResponseWriter, r *http.Request) {
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

	var reports []database.DisasterReport
	var err error

	reporterIDStr := r.URL.Query().Get("reporter_id")
	if reporterIDStr != "" {
		parsed, parseErr := uuid.Parse(reporterIDStr)
		if parseErr != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid reporter_id format")
			return
		}
		reports, err = h.queries.ListDisasterReportsByReporter(r.Context(), database.ListDisasterReportsByReporterParams{
			ReporterID: pgtype.UUID{Bytes: parsed, Valid: true},
			Limit:      limit,
			Offset:     offset,
		})
	} else {
		reports, err = h.queries.ListDisasterReports(r.Context(), database.ListDisasterReportsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve disaster reports")
		return
	}

	res := make([]DisasterReportResponse, 0, len(reports))
	for _, report := range reports {
		res = append(res, DisasterReportResponse{
			ID:         uuid.UUID(report.ID.Bytes).String(),
			ReporterID: uuid.UUID(report.ReporterID.Bytes).String(),
			Location:   report.Location,
			CreatedAt:  report.CreatedAt.Time,
		})
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (h *APIHandler) GetDisasterReportByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid disaster report ID")
		return
	}

	report, err := h.queries.GetDisasterReportByID(r.Context(), pgtype.UUID{Bytes: parsedID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Disaster report not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve disaster report")
		return
	}

	respondWithJSON(w, http.StatusOK, DisasterReportResponse{
		ID:         uuid.UUID(report.ID.Bytes).String(),
		ReporterID: uuid.UUID(report.ReporterID.Bytes).String(),
		Location:   report.Location,
		CreatedAt:  report.CreatedAt.Time,
	})
}
