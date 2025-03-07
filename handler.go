package makaroni

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
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
	Uploader           *Uploader
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

	content := req.Form.Get("content")
	if len(content) == 0 {
		log.Warn("Empty form content")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	keyRaw, keyHtml, keyDelete, err := p.generateKeys()
	if err != nil {
		p.RespondWithError(w, http.StatusInternalServerError, "Failed to generate keys", p.Config)
		return
	}

	metadata := map[string]*string{
		"delete": &keyDelete,
	}

	urlHTML := p.ResultURLPrefix + keyHtml
	urlRaw := p.ResultURLPrefix + keyRaw

	content, file, header, err := p.getFormContent(w, req)
	if err != nil {
		if errors.Is(err, ErrEmptyFormContent) {
			return // getFormContent already handles the redirect
		}
		p.RespondWithError(w, http.StatusInternalServerError, "Failed to get form content", p.Config)
		return
	}

	var html string
	if file != nil {
		html, err = p.processFileUpload(req, file, header, keyRaw, metadata)
	} else {
		html, err = p.processTextUpload(req, content, keyRaw, urlRaw, metadata)
	}

	if err != nil {
		p.RespondWithError(w, http.StatusInternalServerError, "Failed to process upload", p.Config)
		return
	}

	if err = p.Uploader.UploadString(req.Context(), keyHtml, html, contentTypeHTML, metadata); err != nil {
		log.Error("Error uploading HTML: ", err)
		p.RespondWithError(w, http.StatusInternalServerError, "Failed to upload HTML content", p.Config)
		return
	}

	log.Info("Uploaded HTML content with key: ", keyHtml)

	// Set cookie with paste data
	p.setCookies(w, keyRaw, keyHtml, keyDelete)

	p.redirectToURL(w, req, urlHTML)
	log.Debug("Redirecting to URL: ", urlHTML)
}

// handleDeleteRequest handles DELETE requests to remove pastes
func (p *PasteHandler) handleDeleteRequest(w http.ResponseWriter, req *http.Request) {
	// Get all keys from query parameters
	rawKey := req.URL.Query().Get("raw")
	htmlKey := req.URL.Query().Get("html")
	deleteKey := req.URL.Query().Get("key")

	if rawKey == "" || deleteKey == "" || htmlKey == "" {
		log.Warn("Missing required parameters for deletion")
		p.RespondWithError(w, http.StatusBadRequest, "Missing required parameters", p.Config)
		return
	}

	log.Info("Deleting paste with rawKey: ", rawKey, ", using deleteKey: ", deleteKey)

	// Prepare list of keys to delete
	keysToDelete := []string{rawKey, htmlKey}
	for _, key := range keysToDelete {
		metadata, err := p.Uploader.GetMetadata(req.Context(), key)
		if err != nil {
			log.Error("Error retrieving metadata: ", err)
			p.RespondWithError(w, http.StatusNotFound, "Metadata not found", p.Config)
			return
		}

		storedDeleteKey, exists := metadata["delete"]
		if !exists || storedDeleteKey == nil || *storedDeleteKey != deleteKey {
			log.Warn("Invalid delete key provided for: ", rawKey)
			p.RespondWithError(w, http.StatusForbidden, "Invalid delete key", p.Config)
			return
		}
	}

	// Delete all objects in a single batch request
	if err := p.Uploader.DeleteObjects(req.Context(), keysToDelete); err != nil {
		log.Error("Error deleting objects: ", err)
		p.RespondWithError(w, http.StatusInternalServerError, "Failed to delete objects", p.Config)
		return
	}

	log.Info("Successfully deleted paste with key: ", rawKey)
	w.WriteHeader(http.StatusOK)
}

// generateKeys generates unique keys for raw and HTML content
func (p *PasteHandler) generateKeys() (string, string, string, error) {
	uuidV4, err := uuid.NewRandom()
	deleteUuid, err := uuid.NewRandom()
	if err != nil {
		log.Error("Error generating UUID: ", err)
		return "", "", "", err
	}
	keyRaw := uuidV4.String()
	keyHtml := keyRaw + ".html"
	keyDelete := deleteUuid.String()
	return keyRaw, keyHtml, keyDelete, nil
}

// processFileUpload handles file upload and returns the rendered HTML
func (p *PasteHandler) processFileUpload(req *http.Request, file multipart.File, header *multipart.FileHeader, keyRaw string, metadata map[string]*string) (string, error) {
	fileExtension := filepath.Ext(header.Filename)
	contentType := header.Header.Get("Content-Type")

	if len(fileExtension) > 0 {
		keyRaw = keyRaw + fileExtension
	}

	if err := p.Uploader.UploadReader(req.Context(), keyRaw, file, contentType, metadata); err != nil {
		log.Error("Error uploading file: ", err)
		return "", err
	}

	log.Info("Uploaded file with key: ", keyRaw)
	log.Debug("File Size: " + fmt.Sprintf("%d", header.Size))
	log.Debug("MIME Header: " + header.Header.Get("Content-Type"))

	data := FileDownloadData{
		LogoURL:     p.Config.LogoURL,
		IndexURL:    p.Config.IndexURL,
		FaviconURL:  p.Config.FaviconURL,
		FileName:    header.Filename,
		DownloadURL: keyRaw,
		CanView:     CanViewInBrowser(contentType),
	}

	downloadHtml, err := RenderFileDownload(data)
	if err != nil {
		log.Error("Error rendering file download HTML: ", err)
		return "", err
	}

	return string(downloadHtml), nil
}

// processTextUpload handles text content upload and returns the rendered HTML
func (p *PasteHandler) processTextUpload(req *http.Request, content, keyRaw, urlRaw string, metadata map[string]*string) (string, error) {
	syntax := req.Form.Get("syntax")
	if len(syntax) == 0 {
		syntax = "plaintext"
	}
	log.Debug("Using syntax: ", syntax)

	prePageData := PreData{
		LogoURL:     p.Config.LogoURL,
		IndexURL:    p.Config.IndexURL,
		FaviconURL:  p.Config.FaviconURL,
		Content:     "",
		DownloadURL: urlRaw,
	}

	// If content longer than 100 kilobytes, do not highlight it
	if len(content) > 1024*100 {
		log.Debugf("Content size more than 100kb: '%d' bytes, using pre tag", len(content))
		prePageData.Content = content
	} else {
		log.Debugf("Content size: '%d' bytes, highlighting it", len(content))
		highlightBuilder := strings.Builder{}
		if err := highlight(&highlightBuilder, content, syntax, p.Style); err != nil {
			log.Error("Error highlighting content: ", err)
			return "", err
		}
		prePageData.Content = highlightBuilder.String()
	}

	preHtmlPage, err := RenderOutputPre(prePageData)
	if err != nil {
		log.Error("Error generating UUID: ", err)
		RespondServerInternalError(w, err)
		return
	}
	keyRaw := uuidV4.String()
	keyHTML := keyRaw + ".html"
	urlHTML := p.ResultURLPrefix + keyHTML
	urlRaw := p.ResultURLPrefix + keyRaw

	builder := strings.Builder{}
	// todo: use a better templating approach
	builder.Write(p.OutputHTMLPre)
	builder.Write([]byte(fmt.Sprintf("<div class=\"nav\"><a href=\"%s\">raw</a></div>", urlRaw)))
	// if contnent longer 100 kilobytes, do not highlight it
	var html string
	if len(content) > 1024*100 {
		log.Debugf("Content size more than 100kb: '%d' bytes, using pre tag", len(content))
		builder.WriteString("<pre>")
		builder.WriteString(content)
		builder.WriteString("</pre>")
		html = builder.String()
	} else {
		log.Debugf("Content size: '%d' bytes, highlighting it", len(content))
		if err := highlight(&builder, content, syntax, p.Style); err != nil {
			log.Error("Error highlighting content: ", err)
			RespondServerInternalError(w, err)
			return
		}
		html = builder.String()
	}
	if err := p.Upload(keyRaw, content, contentTypeText); err != nil {
		log.Error("Error uploading raw content: ", err)
		RespondServerInternalError(w, err)
		return
	}
	log.Info("Uploaded raw content with key: ", keyRaw)

	if err := p.Upload(keyHTML, html, contentTypeHTML); err != nil {
		log.Error("Error uploading HTML content: ", err)
		RespondServerInternalError(w, err)
		return
	}
	log.Info("Uploaded HTML content with key: ", keyHTML)

	w.Header().Set("Location", urlHTML)
	w.WriteHeader(http.StatusFound)
	log.Debug("Redirecting to URL: ", urlHTML)
}
