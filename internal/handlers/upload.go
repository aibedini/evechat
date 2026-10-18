package handlers

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/logx"
)

var allowedMIME = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"application/pdf": ".pdf",
	"text/plain":      ".txt",
	"application/zip": ".zip",
}

const maxImageEdge = 1600

// Upload accepts an attachment, sniffs its content type and stores it under
// /<project>/<year>/<month>/<ulid><ext>.
func (h *Handlers) Upload(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	maxBytes := int64(h.Cfg.Storage.MaxFileSizeMB) << 20
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+1024)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return apierr.TooLarge(fmt.Sprintf("حجم فایل بیشتر از %d مگابایت است", h.Cfg.Storage.MaxFileSizeMB))
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return apierr.BadRequest("فایلی ارسال نشد")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return apierr.Internal(err)
	}
	if int64(len(data)) > maxBytes {
		return apierr.TooLarge("حجم فایل بیش از حد مجاز است")
	}
	// Sniffing, not the extension, decides the type.
	mime := http.DetectContentType(data)
	if len(data) > 512 {
		mime = http.DetectContentType(data[:512])
	}
	ext, okm := allowedMIME[mime]
	if !okm {
		return apierr.BadRequest("نوع فایل مجاز نیست (تصویر، PDF، متن یا ZIP)")
	}

	var width, height int
	if len(mime) >= 6 && mime[:6] == "image/" && mime != "image/gif" {
		if img, derr := imaging.Decode(bytes.NewReader(data)); derr == nil {
			width, height = img.Bounds().Dx(), img.Bounds().Dy()
			if width > maxImageEdge || height > maxImageEdge {
				resized := imaging.Fit(img, maxImageEdge, maxImageEdge, imaging.Lanczos)
				buf := &bytes.Buffer{}
				format := imaging.JPEG
				if mime == "image/png" {
					format = imaging.PNG
				}
				if eerr := imaging.Encode(buf, resized, format); eerr == nil {
					data = buf.Bytes()
					width, height = resized.Bounds().Dx(), resized.Bounds().Dy()
				}
			}
		}
	} else if mime == "image/gif" {
		if cfg, _, derr := image.DecodeConfig(bytes.NewReader(data)); derr == nil {
			width, height = cfg.Width, cfg.Height
		}
	}

	now := time.Now().UTC()
	name := id.New() + ext
	rel := filepath.Join(agent.ProjectID, now.Format("2006"), now.Format("01"), name)
	dest := filepath.Join(h.Cfg.Storage.UploadsDir, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return apierr.Internal(err)
	}
	if err := os.WriteFile(dest, data, 0o640); err != nil {
		return apierr.Internal(err)
	}
	url := "/uploads/" + filepath.ToSlash(rel)
	logx.WithReq(r.Context()).Info().Str("path", rel).Int64("bytes", int64(len(data))).Msg("upload stored")

	writeJSON(w, http.StatusOK, map[string]any{
		"url":    url,
		"name":   header.Filename,
		"size":   len(data),
		"mime":   mime,
		"width":  width,
		"height": height,
	})
	return nil
}

// UploadsDevFallback serves files when the storage dir is not wired in tests.
func (h *Handlers) UploadsDevFallback() http.Handler {
	return http.NotFoundHandler()
}
