package datasets

import (
	"context"
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/datasets"
	"secure-voting/apps/backend/internal/httpserver/httputil"
	"secure-voting/apps/backend/internal/httpserver/middleware"
)

var listDatasetsFn = func(svc *datasets.Service, ctx context.Context) ([]datasets.ListItem, error) {
	return svc.List(ctx)
}

var getDatasetFn = func(svc *datasets.Service, ctx context.Context, id string) (datasets.Dataset, string, error) {
	return svc.Get(ctx, id)
}

var downloadDatasetFn = func(svc *datasets.Service, ctx context.Context, id string) ([]byte, string, string, string, error) {
	return svc.Download(ctx, id)
}

var exportDatasetFn = func(svc *datasets.Service, ctx context.Context, id string, format string) ([]byte, string, string, string, error) {
	return svc.Export(ctx, id, format)
}

var importDatasetFn = func(svc *datasets.Service, ctx context.Context, meta datasets.ImportMeta, fh *multipart.FileHeader, f multipart.File) (string, string, error) {
	return svc.Import(ctx, meta, fh, f)
}

var generateDatasetFn = func(svc *datasets.Service, ctx context.Context, req datasets.GenerateReq) (string, string, error) {
	return svc.Generate(ctx, req)
}

var exportElectionDatasetFn = func(svc *datasets.Service, ctx context.Context, req datasets.ExportElectionReq) (string, string, error) {
	return svc.ExportElection(ctx, req)
}

type Handlers struct {
	svc *datasets.Service
	cfg config.Config
}

func NewHandlers(svc *datasets.Service, cfg config.Config) *Handlers {
	return &Handlers{svc: svc, cfg: cfg}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	items, err := listDatasetsFn(h.svc, r.Context())
	if err != nil {
		log.Printf("datasets.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list datasets failed")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	ds, code, err := getDatasetFn(h.svc, r.Context(), id)
	if err != nil {
		log.Printf("datasets.get error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "get dataset failed")
		return
	}
	if code != "" {
		switch code {
		case "not_found":
			httputil.WriteError(w, http.StatusNotFound, "not_found", "dataset not found")
		default:
			httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		}
		return
	}
	httputil.WriteJSON(w, http.StatusOK, ds)
}

func (h *Handlers) Download(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	exportFormat := strings.TrimSpace(r.URL.Query().Get("format"))

	if exportFormat != "" {
		data, filename, mime, code, err := exportDatasetFn(h.svc, r.Context(), id, exportFormat)
		if err != nil {
			log.Printf("datasets.export error: %v", err)
			httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "export dataset failed")
			return
		}
		if code != "" {
			writeDownloadDatasetError(w, code)
			return
		}
		httputil.WriteFile(w, filename, mime, data)
		return
	}

	data, filename, mime, code, err := downloadDatasetFn(h.svc, r.Context(), id)
	if err != nil {
		log.Printf("datasets.download error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "download dataset failed")
		return
	}
	if code != "" {
		writeDownloadDatasetError(w, code)
		return
	}
	httputil.WriteFile(w, filename, mime, data)
}

func writeDownloadDatasetError(w http.ResponseWriter, code string) {
	switch code {
	case "not_found":
		httputil.WriteError(w, http.StatusNotFound, "not_found", "dataset not found")
	case "no_ballots":
		httputil.WriteError(w, http.StatusConflict, "no_ballots", "dataset has no stored ballots")
	case "unsupported_export_format":
		httputil.WriteError(w, http.StatusBadRequest, "unsupported_export_format", "unsupported dataset export format")
	default:
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
	}
}

func (h *Handlers) Import(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadBytes)

	if err := r.ParseMultipartForm(h.cfg.MaxUploadBytes); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			httputil.WriteError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "uploaded file is too large")
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid multipart form")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	desc := strings.TrimSpace(r.FormValue("description"))
	format := strings.TrimSpace(r.FormValue("format"))

	fh, err := getFileHeader(r, "file")
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "file is required")
		return
	}

	f, err := fh.Open()
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "cannot open file")
		return
	}
	defer func() { _ = f.Close() }()

	id, code, err := importDatasetFn(h.svc, r.Context(), datasets.ImportMeta{
		Name:        name,
		Description: desc,
		Format:      format,
	}, fh, f)
	if err != nil {
		log.Printf("datasets.import error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "import dataset failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *Handlers) Generate(w http.ResponseWriter, r *http.Request) {
	var req datasets.GenerateReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	id, code, err := generateDatasetFn(h.svc, r.Context(), req)
	if err != nil {
		log.Printf("datasets.generate error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "generate dataset failed")
		return
	}
	if code != "" {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func getFileHeader(r *http.Request, field string) (*multipart.FileHeader, error) {
	_, fh, err := r.FormFile(field)
	if err == nil && fh != nil {
		return fh, nil
	}
	fhs := r.MultipartForm.File[field]
	if len(fhs) == 0 {
		return nil, err
	}
	return fhs[0], nil
}

func (h *Handlers) FromElection(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	role, ok := middleware.RoleFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	var req datasets.ExportElectionReq
	if err := httputil.DecodeJSON(r, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", "invalid json body")
		return
	}

	req.ActorUserID = userID
	req.ActorRole = role

	id, code, err := exportElectionDatasetFn(h.svc, r.Context(), req)
	if err != nil {
		log.Printf("datasets.from-election error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "export election dataset failed")
		return
	}

	if code != "" {
		writeExportElectionDatasetError(w, code)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
}

func writeExportElectionDatasetError(w http.ResponseWriter, code string) {
	switch code {
	case "unauthorized":
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
	case "invalid_election_id":
		httputil.WriteError(w, http.StatusBadRequest, "invalid_election_id", "invalid election_id")
	case "invalid_name":
		httputil.WriteError(w, http.StatusBadRequest, "invalid_name", "invalid dataset name")
	case "invalid_format":
		httputil.WriteError(w, http.StatusBadRequest, "invalid_format", "invalid dataset format")
	case "invalid_candidates":
		httputil.WriteError(w, http.StatusBadRequest, "invalid_candidates", "invalid election candidates")
	case "invalid_ballot_payload":
		httputil.WriteError(w, http.StatusBadRequest, "invalid_ballot_payload", "invalid election ballot payload")
	case "not_found":
		httputil.WriteError(w, http.StatusNotFound, "not_found", "election not found")
	case "forbidden":
		httputil.WriteError(w, http.StatusForbidden, "forbidden", "election dataset export is forbidden")
	case "election_not_ready":
		httputil.WriteError(w, http.StatusConflict, "election_not_ready", "election is not closed yet")
	case "election_not_published":
		httputil.WriteError(w, http.StatusConflict, "election_not_published", "election is not published yet")
	case "aggregates_disabled":
		httputil.WriteError(w, http.StatusForbidden, "aggregates_disabled", "election aggregates are disabled")
	case "no_accepted_ballots":
		httputil.WriteError(w, http.StatusConflict, "no_accepted_ballots", "election has no accepted ballots")
	case "postgres_unavailable":
		httputil.WriteError(w, http.StatusServiceUnavailable, "postgres_unavailable", "postgres connection is unavailable")
	default:
		httputil.WriteError(w, http.StatusBadRequest, "bad_request", code)
	}
}
