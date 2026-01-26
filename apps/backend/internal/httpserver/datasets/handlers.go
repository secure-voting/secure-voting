package datasets

import (
	"log"
	"net/http"
	"strings"
	"mime/multipart"

	"secure-voting/apps/backend/internal/config"
	"secure-voting/apps/backend/internal/datasets"
	"secure-voting/apps/backend/internal/httpserver/httputil"
)

type Handlers struct {
	svc *datasets.Service
	cfg config.Config
}

func NewHandlers(svc *datasets.Service, cfg config.Config) *Handlers {
	return &Handlers{svc: svc, cfg: cfg}
}

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context())
	if err != nil {
		log.Printf("datasets.list error: %v", err)
		httputil.WriteError(w, http.StatusInternalServerError, "internal_error", "list datasets failed")
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	ds, code, err := h.svc.Get(r.Context(), id)
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
	data, filename, mime, code, err := h.svc.Download(r.Context(), id)
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
	defer f.Close()

	id, code, err := h.svc.Import(r.Context(), datasets.ImportMeta{
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

	id, code, err := h.svc.Generate(r.Context(), req)
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
