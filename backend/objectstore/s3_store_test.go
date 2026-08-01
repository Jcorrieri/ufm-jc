package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type fakeS3API struct {
	getOutput   *s3.GetObjectOutput
	getError    error
	copyInput   *s3.CopyObjectInput
	copyError   error
	deleteInput *s3.DeleteObjectInput
	deleteError error
}

func (client *fakeS3API) GetObject(
	context.Context,
	*s3.GetObjectInput,
	...func(*s3.Options),
) (*s3.GetObjectOutput, error) {
	return client.getOutput, client.getError
}

func (client *fakeS3API) CopyObject(
	_ context.Context,
	input *s3.CopyObjectInput,
	_ ...func(*s3.Options),
) (*s3.CopyObjectOutput, error) {
	client.copyInput = input
	return &s3.CopyObjectOutput{}, client.copyError
}

func (client *fakeS3API) DeleteObject(
	_ context.Context,
	input *s3.DeleteObjectInput,
	_ ...func(*s3.Options),
) (*s3.DeleteObjectOutput, error) {
	client.deleteInput = input
	return &s3.DeleteObjectOutput{}, client.deleteError
}

type fakeS3Presigner struct {
	postInput      *s3.PutObjectInput
	postOptions    s3.PresignPostOptions
	downloadInput  *s3.GetObjectInput
	downloadExpiry time.Duration
}

func (presigner *fakeS3Presigner) PresignPostObject(
	_ context.Context,
	input *s3.PutObjectInput,
	options ...func(*s3.PresignPostOptions),
) (*s3.PresignedPostRequest, error) {
	presigner.postInput = input
	for _, option := range options {
		option(&presigner.postOptions)
	}
	return &s3.PresignedPostRequest{
		URL:    "https://bucket.s3.amazonaws.com",
		Values: map[string]string{"key": aws.ToString(input.Key)},
	}, nil
}

func (presigner *fakeS3Presigner) PresignGetObject(
	_ context.Context,
	input *s3.GetObjectInput,
	options ...func(*s3.PresignOptions),
) (*v4.PresignedHTTPRequest, error) {
	presigner.downloadInput = input
	presignOptions := s3.PresignOptions{}
	for _, option := range options {
		option(&presignOptions)
	}
	presigner.downloadExpiry = presignOptions.Expires
	return &v4.PresignedHTTPRequest{URL: "https://download.example"}, nil
}

func TestS3StoreAuthorizesRestrictedPost(t *testing.T) {
	presigner := &fakeS3Presigner{}
	store := newS3Store(&fakeS3API{}, presigner, "image-bucket")
	expiresAt := time.Now().Add(5 * time.Minute)

	authorization, err := store.AuthorizeUpload(
		context.Background(),
		services.UploadAuthorizationRequest{
			ObjectKey:    "staging/user-id/image-id",
			MimeType:     "image/png",
			MaximumBytes: services.MaxImageSize,
			ExpiresAt:    expiresAt,
		},
	)
	if err != nil {
		t.Fatalf("AuthorizeUpload() error = %v", err)
	}

	if authorization.Method != "POST" || authorization.ExpiresAt != expiresAt {
		t.Fatalf("authorization = %#v", authorization)
	}
	if aws.ToString(presigner.postInput.Bucket) != "image-bucket" ||
		aws.ToString(presigner.postInput.Key) != "staging/user-id/image-id" ||
		aws.ToString(presigner.postInput.ContentType) != "image/png" {
		t.Fatalf("post input = %#v", presigner.postInput)
	}
	if presigner.postOptions.Expires > 5*time.Minute ||
		presigner.postOptions.Expires < 4*time.Minute+59*time.Second {
		t.Errorf("POST expiration = %v, want approximately 5m", presigner.postOptions.Expires)
	}
	if len(presigner.postOptions.Conditions) != 2 {
		t.Fatalf("conditions = %#v, want MIME and size restrictions", presigner.postOptions.Conditions)
	}
	if authorization.Fields["key"] != "staging/user-id/image-id" {
		t.Errorf("signed key = %q", authorization.Fields["key"])
	}
	if authorization.Fields["Content-Type"] != "image/png" {
		t.Errorf("signed content type = %q", authorization.Fields["Content-Type"])
	}
}

func TestS3StoreOpensAndPromotesVerifiedObject(t *testing.T) {
	client := &fakeS3API{getOutput: &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader("image")),
		ContentLength: aws.Int64(5),
		ContentType:   aws.String("image/png"),
		ETag:          aws.String("verified-etag"),
	}}
	store := newS3Store(client, &fakeS3Presigner{}, "image-bucket")

	object, err := store.Open(context.Background(), "staging/user/image")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer object.Reader.Close()
	if object.Metadata.SizeBytes != 5 || object.Metadata.MimeType != "image/png" ||
		object.Metadata.Identity != "verified-etag" {
		t.Errorf("metadata = %#v", object.Metadata)
	}

	if err := store.Promote(
		context.Background(),
		"staging/user/image",
		"images/image",
		"verified-etag",
	); err != nil {
		t.Fatalf("Promote() error = %v", err)
	}
	if aws.ToString(client.copyInput.Key) != "images/image" ||
		aws.ToString(client.copyInput.CopySourceIfMatch) != "verified-etag" {
		t.Errorf("copy input = %#v", client.copyInput)
	}
}

func TestS3StoreAuthorizesDownloadAndDeletes(t *testing.T) {
	client := &fakeS3API{}
	presigner := &fakeS3Presigner{}
	store := newS3Store(client, presigner, "image-bucket")

	url, err := store.AuthorizeDownload(
		context.Background(),
		"images/image-id",
		time.Now().Add(5*time.Minute),
	)
	if err != nil || url != "https://download.example" {
		t.Fatalf("AuthorizeDownload() = %q, %v", url, err)
	}
	if presigner.downloadExpiry > 5*time.Minute ||
		presigner.downloadExpiry < 4*time.Minute+59*time.Second {
		t.Errorf("download expiration = %v", presigner.downloadExpiry)
	}

	if err := store.Delete(context.Background(), "images/image-id"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if aws.ToString(client.deleteInput.Key) != "images/image-id" {
		t.Errorf("deleted key = %q", aws.ToString(client.deleteInput.Key))
	}
}

func TestS3StoreClassifiesErrors(t *testing.T) {
	testCases := []struct {
		code string
		want error
	}{
		{code: "NoSuchKey", want: services.ErrObjectNotFound},
		{code: "PreconditionFailed", want: services.ErrObjectChanged},
		{code: "SlowDown", want: services.ErrObjectStoreUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.code, func(t *testing.T) {
			err := classifyError(&smithy.GenericAPIError{Code: testCase.code})
			if !errors.Is(err, testCase.want) {
				t.Fatalf("classifyError() = %v, want %v", err, testCase.want)
			}
		})
	}
}
