package services

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrObjectNotFound = errors.New("object was not found")
	ErrObjectChanged  = errors.New("object changed after verification")
)

type UploadAuthorizationRequest struct {
	ObjectKey    string
	MimeType     string
	MaximumBytes int64
	ExpiresAt    time.Time
}

type UploadAuthorization struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
}

type ObjectMetadata struct {
	SizeBytes int64
	MimeType  string
	Identity  string
}

type StoredObject struct {
	Reader   io.ReadCloser
	Metadata ObjectMetadata
}

type ObjectStore interface {
	AuthorizeUpload(context.Context, UploadAuthorizationRequest) (UploadAuthorization, error)
	Open(context.Context, string) (StoredObject, error)
	Promote(context.Context, string, string, string) error
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

func (UnavailableObjectStore) Open(context.Context, string) (StoredObject, error) {
	return StoredObject{}, ErrObjectStoreUnavailable
}

func (UnavailableObjectStore) Promote(context.Context, string, string, string) error {
	return ErrObjectStoreUnavailable
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
