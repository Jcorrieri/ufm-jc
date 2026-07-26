package services

import (
	"context"
	"io"
	"time"
)

type UploadAuthorizationRequest struct {
	ObjectKey    string
	MimeType     string
	MaximumBytes int64
	ExpiresAt    time.Time
}

type UploadAuthorization struct {
	URL       string            `json:"url"`
	Fields    map[string]string `json:"fields,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type ObjectMetadata struct {
	SizeBytes int64
	MimeType  string
}

type ObjectStore interface {
	AuthorizeUpload(context.Context, UploadAuthorizationRequest) (UploadAuthorization, error)
	Stat(context.Context, string) (ObjectMetadata, error)
	Open(context.Context, string) (io.ReadCloser, error)
	AuthorizeDownload(context.Context, string, time.Time) (string, error)
	Delete(context.Context, string) error
}

// UnavailableObjectStore keeps the API explicit until an adapter is configured.
type UnavailableObjectStore struct{}

func (UnavailableObjectStore) AuthorizeUpload(
	context.Context,
	UploadAuthorizationRequest,
) (UploadAuthorization, error) {
	return UploadAuthorization{}, ErrObjectStoreUnavailable
}

func (UnavailableObjectStore) Stat(context.Context, string) (ObjectMetadata, error) {
	return ObjectMetadata{}, ErrObjectStoreUnavailable
}

func (UnavailableObjectStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, ErrObjectStoreUnavailable
}

func (UnavailableObjectStore) AuthorizeDownload(
	context.Context,
	string,
	time.Time,
) (string, error) {
	return "", ErrObjectStoreUnavailable
}

func (UnavailableObjectStore) Delete(context.Context, string) error {
	return ErrObjectStoreUnavailable
}
