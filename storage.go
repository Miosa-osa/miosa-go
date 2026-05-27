package miosa

import (
	"context"
	"io"
	"strconv"
)

// StorageService provides bucket and object operations for managed S3-compatible storage.
type StorageService struct {
	client *Client
}

// ─── Types ────────────────────────────────────────────────────────────────────

// BucketData is the API representation of a storage bucket.
type BucketData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	TenantID  string `json:"tenant_id"`
	Region    string `json:"region,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// BucketListResponse wraps GET /storage/buckets.
type BucketListResponse struct {
	Data []BucketData `json:"data"`
}

// CreateBucketInput is the request body for POST /storage/buckets.
type CreateBucketInput struct {
	Name           string `json:"name"`
	Region         string `json:"region,omitempty"`
	Public         *bool  `json:"public,omitempty"`
	IdempotencyKey string `json:"-"`
}

// StorageObjectData describes a single object in a bucket.
type StorageObjectData struct {
	Key          string `json:"key"`
	Size         int64  `json:"size"`
	ContentType  string `json:"content_type,omitempty"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// ObjectListResponse wraps GET /storage/buckets/:id/objects.
type ObjectListResponse struct {
	Data   []StorageObjectData `json:"data"`
	Cursor string              `json:"cursor,omitempty"`
}

// ListObjectsInput holds optional query params for listing bucket objects.
type ListObjectsInput struct {
	Prefix string
	Limit  int
	Cursor string
}

// PresignInput is the request body for POST /storage/buckets/:id/presign.
type PresignInput struct {
	Key          string `json:"key"`
	Operation    string `json:"operation"` // "get" | "put"
	ExpiresInSec int    `json:"expires_in_sec"`
	ContentType  string `json:"content_type,omitempty"`
}

// PresignResult is the response from POST /storage/buckets/:id/presign.
type PresignResult struct {
	URL       string            `json:"url"`
	Method    string            `json:"method,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	ExpiresAt string            `json:"expires_at,omitempty"`
}

// ─── Bucket methods ───────────────────────────────────────────────────────────

// ListBuckets returns all storage buckets for the authenticated tenant.
func (s *StorageService) ListBuckets(ctx context.Context) (*BucketListResponse, error) {
	var out BucketListResponse
	if err := s.client.getJSON(ctx, "/storage/buckets", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateBucket provisions a new storage bucket.
func (s *StorageService) CreateBucket(ctx context.Context, input CreateBucketInput) (*BucketData, error) {
	var env apiResponse[BucketData]
	if err := s.client.postJSONIdempotent(ctx, "/storage/buckets", input, &env, idemKey(input.IdempotencyKey)); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// GetBucket fetches a single bucket by ID.
func (s *StorageService) GetBucket(ctx context.Context, bucketID string) (*BucketData, error) {
	var env apiResponse[BucketData]
	if err := s.client.getJSON(ctx, "/storage/buckets/"+bucketID, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// DeleteBucket removes a bucket and all its objects.
func (s *StorageService) DeleteBucket(ctx context.Context, bucketID string) error {
	return s.client.deleteJSON(ctx, "/storage/buckets/"+bucketID, nil)
}

// ─── Object methods ───────────────────────────────────────────────────────────

// ListObjects returns objects in a bucket with optional prefix filtering.
func (s *StorageService) ListObjects(ctx context.Context, bucketID string, input ListObjectsInput) (*ObjectListResponse, error) {
	params := map[string]string{}
	if input.Prefix != "" {
		params["prefix"] = input.Prefix
	}
	if input.Limit > 0 {
		params["limit"] = strconv.Itoa(input.Limit)
	}
	if input.Cursor != "" {
		params["cursor"] = input.Cursor
	}
	var out ObjectListResponse
	if err := s.client.getJSON(ctx, "/storage/buckets/"+bucketID+"/objects"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutObject uploads bytes to bucket/:bucketID/objects/:key.
// contentType defaults to "application/octet-stream" if empty.
func (s *StorageService) PutObject(ctx context.Context, bucketID, key string, content []byte, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	headers := map[string]string{"Content-Type": contentType}
	return s.client.sendJSONWithHeaders(ctx, "PUT", "/storage/buckets/"+bucketID+"/objects/"+key, nil, nil, headers)
}

// PutObjectReader uploads the content of r to bucket/:bucketID/objects/:key.
// contentType defaults to "application/octet-stream" if empty.
func (s *StorageService) PutObjectReader(ctx context.Context, bucketID, key string, r io.Reader, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return s.PutObject(ctx, bucketID, key, data, contentType)
}

// GetObject downloads the raw bytes of an object.
func (s *StorageService) GetObject(ctx context.Context, bucketID, key string) ([]byte, error) {
	data, _, err := s.client.getRaw(ctx, "/storage/buckets/"+bucketID+"/objects/"+key)
	return data, err
}

// DeleteObject removes a single object from a bucket.
func (s *StorageService) DeleteObject(ctx context.Context, bucketID, key string) error {
	return s.client.deleteJSON(ctx, "/storage/buckets/"+bucketID+"/objects/"+key, nil)
}

// Presign mints a signed URL for direct browser upload or download.
// input.Operation is "get" or "put"; ExpiresInSec defaults to 300 when 0.
func (s *StorageService) Presign(ctx context.Context, bucketID string, input PresignInput) (*PresignResult, error) {
	if input.Operation == "" {
		input.Operation = "get"
	}
	if input.ExpiresInSec == 0 {
		input.ExpiresInSec = 300
	}
	var env apiResponse[PresignResult]
	if err := s.client.postJSON(ctx, "/storage/buckets/"+bucketID+"/presign", input, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
