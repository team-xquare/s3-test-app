package service

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.uber.org/zap"
	"s3-test-app/internal/config"
)

// S3Service handles S3 operations
type S3Service struct {
	client *s3.Client
	bucket string
	logger *zap.Logger
}

// File represents a file in S3
type File struct {
	Key          string
	Size         int64
	LastModified string
}

// NewS3Service creates a new S3Service
func NewS3Service(cfg *config.S3Config, logger *zap.Logger) (*S3Service, error) {
	ctx := context.Background()

	sdkConfig, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &S3Service{
		client: client,
		bucket: cfg.Bucket,
		logger: logger,
	}, nil
}

// UploadFile uploads a file to S3
func (s *S3Service) UploadFile(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		s.logger.Error("failed to upload file", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to upload file: %w", err)
	}
	s.logger.Info("file uploaded", zap.String("key", key))
	return nil
}

// ListFiles lists all files in the bucket
func (s *S3Service) ListFiles(ctx context.Context) ([]File, error) {
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		s.logger.Error("failed to list files", zap.Error(err))
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	files := make([]File, 0)
	for _, obj := range result.Contents {
		files = append(files, File{
			Key:          *obj.Key,
			Size:         *obj.Size,
			LastModified: obj.LastModified.Format("2006-01-02 15:04:05"),
		})
	}

	return files, nil
}

// GetFile downloads a file from S3
func (s *S3Service) GetFile(ctx context.Context, key string) ([]byte, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("failed to get file", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		s.logger.Error("failed to read file", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// DeleteFile deletes a file from S3
func (s *S3Service) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		s.logger.Error("failed to delete file", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}
	s.logger.Info("file deleted", zap.String("key", key))
	return nil
}