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

//go:embed resources/error.gohtml
var errorHTML []byte

// IndexData structure for index page
type IndexData struct {
	LogoURL  string
	IndexURL string
	LangList []string
	LogoURL    string
	IndexURL   string
	LangList   []string
	FaviconURL string
}

func renderPage(pageTemplate string, logoURL string, indexURL string) ([]byte, error) {
	tpl, err := template.New("index").Parse(pageTemplate)
	if err != nil {
		log.WithError(err).Error("Failed to parse template")
		return nil, err
	}

	result := strings.Builder{}
	data := IndexData{
		LogoURL:  logoURL,
		IndexURL: indexURL,
		LangList: lexers.Names(false),
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

	return []byte(result.String()), nil
}

func RenderIndexPage(logoURL string, indexURL string) ([]byte, error) {
	return renderPage(string(indexHTML), logoURL, indexURL)
}

func RenderOutputPre(logoURL string, indexURL string) ([]byte, error) {
	return renderPage(string(outputPreHTML), logoURL, indexURL)
}
