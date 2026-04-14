// Package uploads provides image upload APIs using OSS
package uploads

import (
	"context"
	"fmt"
	"io"
	"time"

	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"encore.dev/storage/objects"
)

// UploadBucket is the OSS bucket for uploads
var UploadBucket = objects.NewBucket("uploads", objects.BucketConfig{})

// UploadParams contains upload parameters
type UploadParams struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

// UploadResponse contains upload response
type UploadResponse struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Size     int    `json:"size"`
}

// Image uploads an image to OSS
//encore:api auth method=POST path=/upload.image
func Image(ctx context.Context, params *UploadParams) (*UploadResponse, error) {
	if params.Filename == "" || len(params.Content) == 0 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "filename and content are required",
		}
	}

	// Validate file size (max 10MB)
	if len(params.Content) > 10*1024*1024 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "file size exceeds 10MB limit",
		}
	}

	// Generate unique filename
	userID, _ := auth.UserID()
	timestamp := time.Now().Unix()
	objectKey := fmt.Sprintf("user_%d/%d_%s", userID.Int64(), timestamp, params.Filename)

	// Upload to OSS
	writer, err := UploadBucket.Upload(ctx, objectKey)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to initiate upload",
		}
	}

	if _, err := writer.Write(params.Content); err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to write file",
		}
	}

	if err := writer.Close(); err != nil {
		return nil, &errs.Error{
			Code:    errs.Internal,
			Message: "failed to complete upload",
		}
	}

	// Generate URL
	url := fmt.Sprintf("https://%s/%s", UploadBucket.PublicBaseURL(), objectKey)

	return &UploadResponse{
		URL:      url,
		Filename: params.Filename,
		Size:     len(params.Content),
	}, nil
}

// GetParams contains get parameters
type GetParams struct {
	Key string `json:"key" query:"key"`
}

// Get retrieves an uploaded file
//encore:api auth method=GET path=/upload.get
func Get(ctx context.Context, params *GetParams) ([]byte, error) {
	if params.Key == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "key is required",
		}
	}

	reader, err := UploadBucket.Download(ctx, params.Key)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.NotFound,
			Message: "file not found",
		}
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// Delete deletes an uploaded file
//encore:api auth method=DELETE path=/upload.delete
func Delete(ctx context.Context, params *GetParams) error {
	if params.Key == "" {
		return &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "key is required",
		}
	}

	if err := UploadBucket.Delete(ctx, params.Key); err != nil {
		return &errs.Error{
			Code:    errs.Internal,
			Message: "failed to delete file",
		}
	}

	return nil
}
