package makaroni

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
	"strings"
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
	s3Client *s3.S3
	bucket   string
	config   UploaderConfig
}

// UploadFunc defines the function type for uploading content
type UploadFunc func(key string, content string, contentType string) error

// NewUploader creates and returns a new upload function using S3.
func NewUploader(endpoint string, disableSsl bool, pathStyleAddressing bool, region string, bucket string, keyID string, secret string) (UploadFunc, error) {
	log.Debug("Creating AWS session")
	awsSession, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(keyID, secret, ""),
		Endpoint:         &endpoint,
		Region:           &region,
		S3ForcePathStyle: &pathStyleAddressing,
		DisableSSL:       &disableSsl,
	})
	if err != nil {
		log.Error("Failed to create AWS session: ", err)
		return nil, err
	}
	log.Debug("AWS session created successfully")

	uploader := s3manager.NewUploader(awsSession)
	log.Debug("S3 uploader created successfully")

	upload := func(key string, content string, contentType string) error {
		log.Debugf("Starting upload for key: %s", key)
		output, err := uploader.Upload(&s3manager.UploadInput{
			Bucket:      &bucket,
			Key:         &key,
			Body:        strings.NewReader(content),
			ContentType: &contentType,
		})
		if err != nil {
			log.Error("Upload failed for key: ", key, " error: ", err)
			return err
		}
		log.Debugf("Upload succeeded for key: %s, location: %s", key, output.Location)
		return nil
	}

	return upload, nil
}
