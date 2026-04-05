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

var importDatasetFn = func(svc *datasets.Service, ctx context.Context, meta datasets.ImportMeta, fh *multipart.FileHeader, f multipart.File) (string, string, error) {
	return svc.Import(ctx, meta, fh, f)
}

var generateDatasetFn = func(svc *datasets.Service, ctx context.Context, req datasets.GenerateReq) (string, string, error) {
	return svc.Generate(ctx, req)
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
	data, filename, mime, code, err := downloadDatasetFn(h.svc, r.Context(), id)
	if err != nil {
		log.Printf("datasets.download error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "download dataset failed")
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
	httputil.WriteFile(w, filename, mime, data)
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
