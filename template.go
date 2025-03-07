package makaroni

import (
	"github.com/alecthomas/chroma/lexers"
	log "github.com/sirupsen/logrus"
	"strings"
)

import (
	_ "embed"
	"html/template"
)

//go:embed resources/index.gohtml
var indexHTML []byte

//go:embed resources/pre.gohtml
var outputPreHTML []byte

type IndexData struct {
	LogoURL    string
	IndexURL   string
	LangList   []string
	FaviconURL string
}

func renderPage(pageTemplate string, logoURL string, indexURL string, faviconURL string) ([]byte, error) {
	log.WithField("templateSize", len(pageTemplate)).Debug("Starting template rendering")
	log.Debug("Starting template rendering")

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

	log.Debug("Template rendering completed successfully")
	resultBytes := []byte(result.String())
	log.WithField("resultSize", len(resultBytes)).Debug("Template rendering completed successfully")
	return resultBytes, nil
}

func RenderIndexPage(logoURL string, indexURL string, faviconURL string) ([]byte, error) {
	log.WithFields(log.Fields{
		"logoURL":    logoURL,
		"indexURL":   indexURL,
		"faviconURL": faviconURL,
	}).Debug("Rendering index page template")

	result, err := renderPage(string(indexHTML), logoURL, indexURL, faviconURL)
	if err == nil {
		log.WithField("size", len(result)).Debug("Index page successfully rendered")
	}
	return result, err
}

func RenderOutputPre(logoURL string, indexURL string, faviconURL string) ([]byte, error) {
	log.WithFields(log.Fields{
		"logoURL":    logoURL,
		"indexURL":   indexURL,
		"faviconURL": faviconURL,
	}).Debug("Rendering output pre template")

	result, err := renderPage(string(outputPreHTML), logoURL, indexURL, faviconURL)
	if err == nil {
		log.WithField("size", len(result)).Debug("Output pre template successfully rendered")
	}
	return result, err
}
