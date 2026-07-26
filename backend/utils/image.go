package utils

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

const MaxImageSize = 5 * 1024 * 1024

var (
	ErrImageTooLarge        = errors.New("image exceeds the maximum size")
	ErrInvalidImage         = errors.New("invalid image")
	ErrUnsupportedImageType = errors.New("only JPEG and PNG images are allowed")
	ErrImageSizeMismatch    = errors.New("image size does not match the expected size")
	ErrImageMimeMismatch    = errors.New("image MIME type does not match the expected type")
	ErrImageDimensions      = errors.New("image dimensions exceed the allowed maximum")
)

type VerificationOptions struct {
	MaximumSizeBytes  int64
	ExpectedSizeBytes int64
	ExpectedMimeType  string
}

type VerifiedImage struct {
	SizeBytes      int64
	MimeType       string
	Width          int
	Height         int
	ChecksumSHA256 string
}

type ImageVerifier interface {
	Verify(
		ctx context.Context,
		reader io.Reader,
		options VerificationOptions,
	) (VerifiedImage, error)
}

type StandardImageVerifier struct {
	maxWidth  int
	maxHeight int
	maxPixels int64
}

func NewStandardImageVerifier(maxWidth, maxHeight int, maxPixels int64) ImageVerifier {
	return &StandardImageVerifier{
		maxWidth:  maxWidth,
		maxHeight: maxHeight,
		maxPixels: maxPixels,
	}
}

func (verifier *StandardImageVerifier) Verify(
	ctx context.Context,
	reader io.Reader,
	options VerificationOptions,
) (VerifiedImage, error) {
	maximumSize := options.MaximumSizeBytes
	if maximumSize <= 0 {
		maximumSize = MaxImageSize
	}

	data, err := io.ReadAll(
		io.LimitReader(contextReader{ctx: ctx, reader: reader}, maximumSize+1),
	)
	if err != nil {
		return VerifiedImage{}, fmt.Errorf("read image: %w", err)
	}
	if int64(len(data)) > maximumSize {
		return VerifiedImage{}, ErrImageTooLarge
	}
	if options.ExpectedSizeBytes > 0 &&
		int64(len(data)) != options.ExpectedSizeBytes {
		return VerifiedImage{}, ErrImageSizeMismatch
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return VerifiedImage{}, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	mimeType, err := mimeTypeForFormat(format)
	if err != nil {
		return VerifiedImage{}, err
	}
	if options.ExpectedMimeType != "" &&
		mimeType != options.ExpectedMimeType {
		return VerifiedImage{}, ErrImageMimeMismatch
	}
	if (verifier.maxWidth > 0 && config.Width > verifier.maxWidth) ||
		(verifier.maxHeight > 0 && config.Height > verifier.maxHeight) {
		return VerifiedImage{}, ErrImageDimensions
	}
	pixelCount := int64(config.Width) * int64(config.Height)
	if verifier.maxPixels > 0 && pixelCount > verifier.maxPixels {
		return VerifiedImage{}, ErrImageDimensions
	}

	// Decode the complete image so a valid header with a truncated body is not
	// accepted as a verified object.
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return VerifiedImage{}, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	checksum := sha256.Sum256(data)
	return VerifiedImage{
		SizeBytes:      int64(len(data)),
		MimeType:       mimeType,
		Width:          config.Width,
		Height:         config.Height,
		ChecksumSHA256: hex.EncodeToString(checksum[:]),
	}, nil
}

func mimeTypeForFormat(format string) (string, error) {
	switch format {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	default:
		return "", ErrUnsupportedImageType
	}
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(data []byte) (int, error) {
	select {
	case <-reader.ctx.Done():
		return 0, reader.ctx.Err()
	default:
		return reader.reader.Read(data)
	}
}
