package makaroni

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
)

// UploaderConfig contains settings for the uploader
type UploaderConfig struct {
	Endpoint            string
	DisableSSL          bool
	PathStyleAddressing bool
	Region              string
	Bucket              string
	KeyID               string
	Secret              string
	PartSize            int64         // Part size in bytes for multipart uploads
	Concurrency         int           // Number of concurrent uploads
	Timeout             time.Duration // Timeout for operations
}

// Uploader provides methods for uploading data to S3
type Uploader struct {
	uploader *s3manager.Uploader
	bucket   string
	config   UploaderConfig
}

// UploadFunc defines the function type for uploading content
type UploadFunc func(key string, content string, contentType string) error

// NewUploader creates a new uploader instance
func NewUploader(config UploaderConfig) (*Uploader, error) {
	log.Info("Creating AWS session")

	// Set default values if not specified
	if config.PartSize == 0 {
		config.PartSize = s3manager.DefaultUploadPartSize
	}
	if config.Concurrency == 0 {
		config.Concurrency = s3manager.DefaultUploadConcurrency
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	awsSession, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.KeyID, config.Secret, ""),
		Endpoint:         &config.Endpoint,
		Region:           &config.Region,
		S3ForcePathStyle: &config.PathStyleAddressing,
		DisableSSL:       &config.DisableSSL,
	})
	if err != nil {
		log.Error("Failed to create AWS session: ", err)
		return nil, err
	}
	log.Info("AWS session created successfully")

	uploader := s3manager.NewUploader(awsSession, func(u *s3manager.Uploader) {
		u.PartSize = config.PartSize
		u.Concurrency = config.Concurrency
	})
	log.Info("S3 uploader created successfully")

	return &Uploader{
		uploader: uploader,
		bucket:   config.Bucket,
		config:   config,
	}, nil
}

// UploadString uploads string content to S3
func (u *Uploader) UploadString(ctx context.Context, key string, content string, contentType string, metadata map[string]*string) error {
	return u.UploadReader(ctx, key, strings.NewReader(content), contentType, metadata)
}

// UploadReader uploads data from io.Reader to S3
func (u *Uploader) UploadReader(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]*string) error {
	log.Debugf("Starting upload for key: %s", key)

	// Create a context with timeout if context is not set
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), u.config.Timeout)
		defer cancel()
	}

	input := &s3manager.UploadInput{
		Bucket:      &u.bucket,
		Key:         &key,
		Body:        reader,
		ContentType: &contentType,
	}

	if metadata != nil {
		input.Metadata = metadata
	}

	output, err := u.uploader.UploadWithContext(ctx, input)
	if err != nil {
		log.Error("Upload failed for key: ", key, " error: ", err)
		return err
	}
	log.Debugf("Upload succeeded for key: %s, location: %s", key, output.Location)
	return nil
}
