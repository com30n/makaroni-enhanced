package makaroni

import (
	"github.com/alecthomas/chroma/lexers"
	log "github.com/sirupsen/logrus"
	"strings"
)

import (
	_ "embed"
	"text/template"
)

//go:embed resources/index.gohtml
var indexHTML []byte

//go:embed resources/pre.gohtml
var outputPreHTML []byte

//go:embed resources/fileDownload.gohtml
var fileDownloadHTML []byte

type IndexData struct {
	LogoURL    string
	IndexURL   string
	LangList   []string
	FaviconURL string
}
type FileDownloadData struct {
	LogoURL     string
	IndexURL    string
	FaviconURL  string
	FileName    string
	DownloadURL string
}

type PreData struct {
	LogoURL     string
	IndexURL    string
	FaviconURL  string
	Content     string
	DownloadURL string
}

func renderPageWithData(pageTemplate string, data interface{}) ([]byte, error) {
	log.WithFields(log.Fields{
		"templateSize": len(pageTemplate),
		"data":         data,
	}).Debug("Starting template rendering")

	tpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse template")
		return nil, err
	}

	result := strings.Builder{}

	log.Debug("Executing template with data")

	if err := tpl.Execute(&result, data); err != nil {
		log.WithError(err).Error("Failed to execute template")
		return nil, err
	}

	resultBytes := []byte(result.String())
	log.WithField("resultSize", len(resultBytes)).Debug("Template rendering completed successfully")
	return resultBytes, nil
}

func renderPage(pageTemplate string, logoURL string, indexURL string, faviconURL string) ([]byte, error) {
	log.WithField("templateSize", len(pageTemplate)).Debug("Starting template rendering")

	tpl, err := template.New("index").Parse(pageTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse template")
		return nil, err
	}

	result := strings.Builder{}
	data := IndexData{
		LogoURL:    logoURL,
		IndexURL:   indexURL,
		LangList:   lexers.Names(false),
		FaviconURL: faviconURL,
	}

	log.WithFields(log.Fields{
		"logoURL":    logoURL,
		"indexURL":   indexURL,
		"faviconURL": faviconURL,
		"languages":  len(data.LangList),
	}).Debug("Executing template with data")

	if err := tpl.Execute(&result, &data); err != nil {
		log.WithError(err).Error("Failed to execute template")
		return nil, err
	}

	resultBytes := []byte(result.String())
	log.WithField("resultSize", len(resultBytes)).Debug("Template rendering completed successfully")
	return resultBytes, nil
}

func RenderIndexPage(logoURL string, indexURL string, faviconURL string) ([]byte, error) {
	log.Info("Rendering index page")
	result, err := renderPage(string(indexHTML), logoURL, indexURL, faviconURL)
	if err == nil {
		log.WithField("size", len(result)).Debug("Index page successfully rendered")
	}
	return result, err
}

func RenderOutputPre(data PreData) ([]byte, error) {
	log.Info("Rendering output page template")

	result, err := renderPageWithData(string(outputPreHTML), &data)
	if err == nil {
		log.WithField("size", len(result)).Debug("Output pre template successfully rendered")
	}
	return result, err
}

func RenderFileDownload(data FileDownloadData) ([]byte, error) {
	log.Info("Rendering output pre HTML", data.FileName)

	result, err := renderPageWithData(string(fileDownloadHTML), &data)
	if err == nil {
		log.WithField("size", len(result)).Debug("Output pre template successfully rendered")
	}
	return result, err
}
