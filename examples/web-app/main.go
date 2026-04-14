// Example Web Application for Infracast
// This demonstrates a simple web frontend with object storage integration
package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"encore.dev/storage/objects"
)

// assetsBucket is the object storage bucket (managed by Infracast)
var assetsBucket = objects.NewBucket("assets", objects.BucketConfig{
	Public: false,
})

// PageData represents data passed to HTML templates
type PageData struct {
	Title   string
	Message string
	Assets  []Asset
}

// Asset represents a stored asset
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// HomePage renders the home page
//encore:api public path=/
func HomePage(ctx context.Context) (*HTMLResponse, error) {
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Web App Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 800px; margin: 0 auto; }
        h1 { color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Welcome to Infracast Web App Example</h1>
        <p>This app demonstrates web frontend deployment with Infracast.</p>
        <p>Environment: ` + os.Getenv("INFRA_ENV") + `</p>
    </div>
</body>
</html>
`
	return &HTMLResponse{Body: html}, nil
}

// HTMLResponse represents an HTML response
type HTMLResponse struct {
	Body string
}

func (r *HTMLResponse) MarshalJSON() ([]byte, error) {
	return []byte(`"` + r.Body + `"`), nil
}

// UploadAsset handles file uploads to object storage
//encore:api public method=POST path=/upload
func UploadAsset(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	// Upload to object storage
	obj, err := assetsBucket.Upload(ctx, req.Filename, req.Content)
	if err != nil {
		return nil, err
	}

	return &UploadResponse{
		Name: obj.Name,
		URL:  obj.URL,
		Size: obj.Size,
	}, nil
}

// UploadRequest represents a file upload request
type UploadRequest struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

// UploadResponse represents a file upload response
type UploadResponse struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// ListAssets returns all assets in the bucket
//encore:api public path=/assets
func ListAssets(ctx context.Context) (*ListAssetsResponse, error) {
	// List objects in bucket
	objs, err := assetsBucket.List(ctx)
	if err != nil {
		return nil, err
	}

	var assets []Asset
	for _, obj := range objs {
		assets = append(assets, Asset{
			Name: obj.Name,
			URL:  obj.URL,
			Size: obj.Size,
		})
	}

	return &ListAssetsResponse{Assets: assets}, nil
}

// ListAssetsResponse represents the list assets response
type ListAssetsResponse struct {
	Assets []Asset `json:"assets"`
}

// HealthCheck returns the service health status
//encore:api public path=/health
func HealthCheck(ctx context.Context) (*HealthResponse, error) {
	return &HealthResponse{
		Status: "ok",
		Env:    os.Getenv("INFRA_ENV"),
	}, nil
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

func main() {
	log.Println("Web App starting...")
	// Encore handles the server lifecycle
}
