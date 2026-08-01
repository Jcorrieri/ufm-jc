package objectstore

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Jcorrieri/uf-marketplace/backend/services"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type s3API interface {
	GetObject(
		context.Context,
		*s3.GetObjectInput,
		...func(*s3.Options),
	) (*s3.GetObjectOutput, error)
	CopyObject(
		context.Context,
		*s3.CopyObjectInput,
		...func(*s3.Options),
	) (*s3.CopyObjectOutput, error)
	DeleteObject(
		context.Context,
		*s3.DeleteObjectInput,
		...func(*s3.Options),
	) (*s3.DeleteObjectOutput, error)
}

type s3Presigner interface {
	PresignPostObject(
		context.Context,
		*s3.PutObjectInput,
		...func(*s3.PresignPostOptions),
	) (*s3.PresignedPostRequest, error)
	PresignGetObject(
		context.Context,
		*s3.GetObjectInput,
		...func(*s3.PresignOptions),
	) (*v4.PresignedHTTPRequest, error)
}

// S3Store persists image bytes in one private S3 bucket.
type S3Store struct {
	client    s3API
	presigner s3Presigner
	bucket    string
}

var _ services.ObjectStore = (*S3Store)(nil)

func NewS3Store(client *s3.Client, bucket string) *S3Store {
	return newS3Store(client, s3.NewPresignClient(client), bucket)
}

func newS3Store(client s3API, presigner s3Presigner, bucket string) *S3Store {
	return &S3Store{client: client, presigner: presigner, bucket: bucket}
}

func (store *S3Store) AuthorizeUpload(
	ctx context.Context,
	request services.UploadAuthorizationRequest,
) (services.UploadAuthorization, error) {
	expires := time.Until(request.ExpiresAt)
	if expires <= 0 {
		return services.UploadAuthorization{}, services.ErrInvalidImageMetadata
	}

	result, err := store.presigner.PresignPostObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:      aws.String(store.bucket),
			Key:         aws.String(request.ObjectKey),
			ContentType: aws.String(request.MimeType),
		},
		func(options *s3.PresignPostOptions) {
			options.Expires = expires
			options.Conditions = []any{
				map[string]string{"Content-Type": request.MimeType},
				[]any{"content-length-range", int64(1), request.MaximumBytes},
			}
		},
	)
	if err != nil {
		return services.UploadAuthorization{}, classifyError(err)
	}
	// The SDK signs custom POST conditions but does not add their matching form
	// values. S3 requires every exact-match condition to be present in the form.
	result.Values["Content-Type"] = request.MimeType

	return services.UploadAuthorization{
		URL:       result.URL,
		Method:    "POST",
		Fields:    result.Values,
		ExpiresAt: request.ExpiresAt,
	}, nil
}

func (store *S3Store) Open(ctx context.Context, objectKey string) (
	services.StoredObject,
	error,
) {
	result, err := store.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return services.StoredObject{}, classifyError(err)
	}

	return services.StoredObject{
		Reader: result.Body,
		Metadata: services.ObjectMetadata{
			SizeBytes: aws.ToInt64(result.ContentLength),
			MimeType:  aws.ToString(result.ContentType),
			Identity:  aws.ToString(result.ETag),
		},
	}, nil
}

func (store *S3Store) Promote(
	ctx context.Context,
	sourceKey string,
	destinationKey string,
	identity string,
) error {
	_, err := store.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(store.bucket),
		Key:               aws.String(destinationKey),
		CopySource:        aws.String(url.PathEscape(store.bucket + "/" + sourceKey)),
		CopySourceIfMatch: aws.String(identity),
	})
	return classifyError(err)
}

func (store *S3Store) AuthorizeDownload(
	ctx context.Context,
	objectKey string,
	expiresAt time.Time,
) (string, error) {
	expires := time.Until(expiresAt)
	if expires <= 0 {
		return "", services.ErrInvalidImageMetadata
	}
	result, err := store.presigner.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(store.bucket),
			Key:    aws.String(objectKey),
		},
		func(options *s3.PresignOptions) { options.Expires = expires },
	)
	if err != nil {
		return "", classifyError(err)
	}
	return result.URL, nil
}

func (store *S3Store) Delete(ctx context.Context, objectKey string) error {
	_, err := store.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(objectKey),
	})
	return classifyError(err)
}

func classifyError(err error) error {
	if err == nil {
		return nil
	}

	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch strings.ToLower(apiError.ErrorCode()) {
		case "nosuchkey", "notfound", "404":
			return fmt.Errorf("%w: %v", services.ErrObjectNotFound, err)
		case "preconditionfailed", "412":
			return fmt.Errorf("%w: %v", services.ErrObjectChanged, err)
		}
	}

	return fmt.Errorf("%w: %v", services.ErrObjectStoreUnavailable, err)
}
