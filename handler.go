package makaroni

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	pasteDataCookieName = "paste_data"
	cookieMaxAge        = 86400 * 365 // 365 days
	contentTypeHTML     = "text/html"
	contentTypeText     = "text/plain"
)

var (
	ErrEmptyFormContent = errors.New("empty form content")
)

// PasteHandler structure for handling uploads
type PasteHandler struct {
	IndexHTML          []byte
	Upload             func(key string, content string, contentType string) error
	Style              string
	ResultURLPrefix    string
	MultipartMaxMemory int64
	Config             *Config
}

// RespondServerInternalError sends a response with status 500 and logs the error.
func RespondServerInternalError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	log.Error(err)
}

// ServeHTTP handles HTTP requests using different log levels.
func (p *PasteHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Info("Received request: ", req.Method, " ", req.URL.Path)

	if req.Method == http.MethodGet {
		log.Info("Sending index page")
		w.Header().Set("Content-Type", contentTypeHTML)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(p.IndexHTML); err != nil {
			log.Error("Error sending indexHTML: ", err)
		}
		return
	}

	if req.Method != http.MethodPost {
		log.Warn("Unsupported request method: ", req.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := req.ParseMultipartForm(p.MultipartMaxMemory); err != nil {
		log.Warn("Error parsing form: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uuidV4, err := uuid.NewRandom()
	if err != nil {
		log.Error("Error generating UUID: ", err)
		RespondServerInternalError(w, err)
		return
	}

	keyRaw := uuidV4.String()
	keyHtml := keyRaw + ".html"

	var fileExtension string
	var fileContentType string

	content := req.Form.Get("content")
	file, header, err := req.FormFile("file")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		log.Warn("Error retrieving the file: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if file != nil {
		defer func(file multipart.File) {
			err := file.Close()
			if err != nil {
				log.Error("Error closing file: ", err)
			}
		}(file)
		fileExtension = filepath.Ext(header.Filename)
		fileContentType = header.Header.Get("Content-Type")

		log.Debug("Uploaded File: " + header.Filename)
		log.Debug("File Size: " + fmt.Sprintf("%d", header.Size))
		log.Debug("MIME Header: " + header.Header.Get("Content-Type"))

		fileContent, err := io.ReadAll(file)
		if err != nil {
			log.Error("Error reading file: ", err)
			RespondServerInternalError(w, err)
			return
		}
		content = string(fileContent)
	}

	if len(content) == 0 {
		log.Warn("Empty form content")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(fileExtension) > 0 {
		keyRaw = keyRaw + fileExtension
	}

	urlHTML := p.ResultURLPrefix + keyHtml
	urlRaw := p.ResultURLPrefix + keyRaw

	var html string
	builder := strings.Builder{}

	if len(fileExtension) > 0 {
		if err := p.Upload(keyRaw, content, fileContentType); err != nil {
			log.Error("Error uploading file: ", err)
			RespondServerInternalError(w, err)
			return
		}
		log.Info("Uploaded file with key: ", keyRaw)
		data := FileDownloadData{
			p.Config.LogoURL,
			p.Config.IndexURL,
			p.Config.FaviconURL,
			header.Filename,
			urlRaw,
		}
		downloadHtml, err := RenderFileDownload(data)
		if err != nil {
			log.Error("Error rendering file download HTML: ", err)
			RespondServerInternalError(w, err)
			return
		}

		builder.Write(downloadHtml)
		html = builder.String()
	} else {
		syntax := req.Form.Get("syntax")
		if len(syntax) == 0 {
			syntax = "plaintext"
		}
		log.Debug("Using syntax: ", syntax)

		prePageData := PreData{
			p.Config.LogoURL,
			p.Config.IndexURL,
			p.Config.FaviconURL,
			"",
			urlRaw,
		}

		// if content longer 100 kilobytes, do not highlight it
		if len(content) > 1024*100 {
			log.Debugf("Content size more than 100kb: '%d' bytes, using pre tag", len(content))
			prePageData.Content = content
		} else {
			log.Debugf("Content size: '%d' bytes, highlighting it", len(content))
			highlightBuilder := strings.Builder{}
			if err := highlight(&highlightBuilder, content, syntax, p.Style); err != nil {
				log.Error("Error highlighting content: ", err)
				RespondServerInternalError(w, err)
				return
			}
			prePageData.Content = highlightBuilder.String()
		}

		preHtmlPage, err := RenderOutputPre(prePageData)
		if err != nil {
			log.Error("Error rendering output pre HTML: ", err)
			RespondServerInternalError(w, err)
			return
		}
		builder.Write(preHtmlPage)
		html = builder.String()

		if err := p.Upload(keyRaw, content, contentTypeText); err != nil {
			log.Error("Error uploading raw content: ", err)
			RespondServerInternalError(w, err)
			return
		}
		log.Info("Uploaded raw content with key: ", keyRaw)

	}
	if err := p.Upload(keyHtml, html, contentTypeHTML); err != nil {
		log.Error("Error uploading HTML content: ", err)
		RespondServerInternalError(w, err)
		return
	}
	log.Info("Uploaded HTML content with key: ", keyHtml)

	w.Header().Set("Location", urlHTML)
	w.WriteHeader(http.StatusFound)
	log.Debug("Redirecting to URL: ", urlHTML)
}
