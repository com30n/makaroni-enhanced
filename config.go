package makaroni

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"reflect"
	"strings"
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
func BindEnvVars(config Config) {
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
