package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/kaero/makaroni"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	Address            string `mapstructure:"address"`
	MultipartMaxMemory int64  `mapstructure:"multipart_max_memory"`

	// URLs
	IndexURL        string `mapstructure:"index_url"`
	ResultURLPrefix string `mapstructure:"result_url_prefix"`
	LogoURL         string `mapstructure:"logo_url"`
	FaviconURL      string `mapstructure:"favicon_url"`
	Style           string `mapstructure:"style"`

	// S3 settings
	S3Endpoint   string `mapstructure:"s3_endpoint"`
	S3Region     string `mapstructure:"s3_region"`
	S3Bucket     string `mapstructure:"s3_bucket"`
	S3KeyID      string `mapstructure:"s3_key_id"`
	S3SecretKey  string `mapstructure:"s3_secret_key"`
	S3PathStyle  bool   `mapstructure:"s3_path_style"`
	S3DisableSSL bool   `mapstructure:"s3_disable_ssl"`
}

// Bind environment variables automatically
func bindEnvVars(config Config) {
	t := reflect.TypeOf(config)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		envKey := field.Tag.Get("mapstructure")
		if envKey != "" {
			err := viper.BindEnv(envKey)
			if err != nil {
				log.Errorf("Error binding environment variable %s: %v", envKey, err)
			}
		}
	}
}

// MaskSecret hides part of a secret value for safe logging
func MaskSecret(secret string) string {
	if len(secret) <= 6 {
		if len(secret) <= 2 {
			return secret
		}
		return secret[:1] + strings.Repeat("*", len(secret)-2) + secret[len(secret)-1:]
	}
	return secret[:3] + strings.Repeat("*", len(secret)-6) + secret[len(secret)-3:]
}

// LogConfig logs configuration settings while hiding secrets
func LogConfig() {
	categories := map[string][]string{
		"Server": {"address", "multipart_max_memory"},
		"URL":    {"index_url", "result_url_prefix", "logo_url", "favicon_url", "style"},
		"S3":     {"s3_endpoint", "s3_region", "s3_bucket", "s3_key_id", "s3_secret_key", "s3_path_style", "s3_disable_ssl"},
	}

	for category, keys := range categories {
		log.Debugf("%s settings:", category)
		for _, key := range keys {
			value := viper.Get(key)
			if key == "s3_secret_key" && value != nil {
				valueStr, ok := value.(string)
				if ok && valueStr != "" {
					log.Debugf("  MKRN_%s: %s", strings.ToUpper(key), MaskSecret(valueStr))
					continue
				}
			}
			log.Debugf("  MKRN_%s: %v", strings.ToUpper(key), value)
		}
	}
}

func InitLogger() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	logLevel := os.Getenv("LOG_LEVEL")
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		level = log.InfoLevel
		log.Warn("Invalid LOG_LEVEL, defaulting to Info level, supported levels: ", log.AllLevels)
	}
	log.SetLevel(level)
}

func main() {
	var config Config
	var rootCmd = &cobra.Command{
		Use:   "makaroni",
		Short: "Makaroni is a paste service",
		Long:  "A web service for sharing code snippets with syntax highlighting",
		Run: func(cmd *cobra.Command, args []string) {
			if err := viper.Unmarshal(&config); err != nil {
				log.Fatalf("Error parsing configuration: %v", err)
			}

			LogConfig()

			server, err := SetupServer(&config)
			if err != nil {
				log.Fatalf("Error setting up server: %v", err)
			}

			// Start server in a goroutine
			go func() {
				log.Infof("Server started on address %s", server.Addr)
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Fatalf("Server error: %v", err)
				}
			}()

			// Setup graceful shutdown
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
			<-stop

			log.Info("Shutdown signal received, stopping server...")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				log.Errorf("Server shutdown error: %v", err)
			}

			log.Info("Server stopped successfully")
		},
	}

	// Initialize configuration
	InitLogger()
	log.Debug("Application starting (debug level)")

	// Define flags
	flags := rootCmd.Flags()
	flags.String("address", "", "Address to serve")
	flags.Int64("multipart-max-memory", 0, "Maximum memory for multipart forms")
	flags.String("index-url", "", "URL to the index page")
	flags.String("result-url-prefix", "", "Upload result URL prefix")
	flags.String("logo-url", "", "Logo URL for the form page")
	flags.String("favicon-url", "", "Favicon URL")
	flags.String("style", "", "Formatting style")
	flags.String("s3-endpoint", "", "S3 endpoint")
	flags.String("s3-region", "", "S3 region")
	flags.String("s3-bucket", "", "S3 bucket")
	flags.String("s3-key-id", "", "S3 key ID")
	flags.String("s3-secret-key", "", "S3 secret key")
	flags.Bool("s3-path-style", false, "S3 use path style addressing")
	flags.Bool("s3-disable-ssl", false, "S3 disable SSL")

	// Bind flags with viper
	if err := viper.BindPFlags(flags); err != nil {
		log.Fatalf("Error binding flags: %v", err)
	}

	// Setup Viper for environment variables
	viper.SetEnvPrefix("MKRN")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Bind environment variables automatically
	bindEnvVars(config)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// SetupServer creates and configures HTTP server
func SetupServer(config *Config) (*http.Server, error) {
	// Render HTML templates
	log.Info("Rendering index page")
	indexHTML, err := makaroni.RenderIndexPage(config.LogoURL, config.IndexURL, config.FaviconURL)
	if err != nil {
		return nil, err
	}

	log.Info("Rendering output pre HTML")
	outputPreHTML, err := makaroni.RenderOutputPre(config.LogoURL, config.IndexURL, config.FaviconURL)
	if err != nil {
		log.Warnf("Error rendering output pre HTML: %v", err)
		return nil, err
	}

	// Initialize S3 uploader
	log.Info("Initializing uploader")
	uploadFunc, err := makaroni.NewUploader(
		config.S3Endpoint,
		config.S3DisableSSL,
		config.S3PathStyle,
		config.S3Region,
		config.S3Bucket,
		config.S3KeyID,
		config.S3SecretKey,
	)
	if err != nil {
		return nil, err
	}

	// Setup routing
	fileServer := http.FileServer(http.Dir("./resources/static"))
	mux := http.NewServeMux()

	// Handle static files
	mux.Handle("/static/", LogStaticFileRequest(http.StripPrefix("/static/", fileServer)))

	// Main handler
	mux.Handle("/", &makaroni.PasteHandler{
		IndexHTML:          indexHTML,
		OutputHTMLPre:      outputPreHTML,
		Upload:             uploadFunc,
		ResultURLPrefix:    config.ResultURLPrefix,
		Style:              config.Style,
		MultipartMaxMemory: config.MultipartMaxMemory,
	})

	return &http.Server{
		Addr:    config.Address,
		Handler: mux,
	}, nil
}

// LogStaticFileRequest middleware for logging requests to static files
func LogStaticFileRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Received request: ", r.Method, " ", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
