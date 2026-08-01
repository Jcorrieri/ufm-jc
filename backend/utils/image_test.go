package utils_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/Jcorrieri/uf-marketplace/backend/utils"
)

func TestStandardImageVerifierVerifiesPNGMetadata(t *testing.T) {
	data := encodeImage(t, "png", 3, 2)
	verifier := utils.NewStandardImageVerifier(10, 10, 100)

	result, err := verifier.Verify(context.Background(), bytes.NewReader(data),
		utils.VerificationOptions{
			MaximumSizeBytes:  int64(len(data)),
			ExpectedSizeBytes: int64(len(data)),
			ExpectedMimeType:  "image/png",
		})
	if err != nil {
		t.Fatalf("Verify returned an unexpected error: %v", err)
	}

	checksum := sha256.Sum256(data)
	if result.SizeBytes != int64(len(data)) {
		t.Errorf("SizeBytes = %d, want %d", result.SizeBytes, len(data))
	}
	if result.MimeType != "image/png" {
		t.Errorf("MimeType = %q, want image/png", result.MimeType)
	}
	if result.Width != 3 || result.Height != 2 {
		t.Errorf("dimensions = %dx%d, want 3x2", result.Width, result.Height)
	}
	if result.ChecksumSHA256 != hex.EncodeToString(checksum[:]) {
		t.Errorf("ChecksumSHA256 = %q, want %x", result.ChecksumSHA256, checksum)
	}
}

func TestStandardImageVerifierAcceptsJPEG(t *testing.T) {
	data := encodeImage(t, "jpeg", 2, 2)
	verifier := utils.NewStandardImageVerifier(10, 10, 100)

	result, err := verifier.Verify(context.Background(), bytes.NewReader(data),
		utils.VerificationOptions{ExpectedMimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("Verify returned an unexpected error: %v", err)
	}
	if result.MimeType != "image/jpeg" {
		t.Errorf("MimeType = %q, want image/jpeg", result.MimeType)
	}
}

func TestStandardImageVerifierRejectsInvalidInput(t *testing.T) {
	pngData := encodeImage(t, "png", 2, 2)
	tests := []struct {
		name          string
		data          []byte
		options       utils.VerificationOptions
		maxWidth      int
		maxHeight     int
		maxPixels     int64
		expectedError error
	}{
		{
			name:          "too large",
			data:          pngData,
			options:       utils.VerificationOptions{MaximumSizeBytes: int64(len(pngData) - 1)},
			expectedError: utils.ErrImageTooLarge,
		},
		{
			name:          "size mismatch",
			data:          pngData,
			options:       utils.VerificationOptions{ExpectedSizeBytes: int64(len(pngData) + 1)},
			expectedError: utils.ErrImageSizeMismatch,
		},
		{
			name: "MIME mismatch",
			data: pngData,
			options: utils.VerificationOptions{
				ExpectedMimeType: "image/jpeg",
			},
			expectedError: utils.ErrImageMimeMismatch,
		},
		{
			name:          "malformed data",
			data:          []byte("not an image"),
			expectedError: utils.ErrInvalidImage,
		},
		{
			name:          "dimensions too large",
			data:          pngData,
			maxWidth:      1,
			maxHeight:     10,
			expectedError: utils.ErrImageDimensions,
		},
		{
			name:          "pixel count too large",
			data:          pngData,
			maxWidth:      10,
			maxHeight:     10,
			maxPixels:     3,
			expectedError: utils.ErrImageDimensions,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := utils.NewStandardImageVerifier(
				test.maxWidth,
				test.maxHeight,
				test.maxPixels,
			)
			_, err := verifier.Verify(
				context.Background(),
				bytes.NewReader(test.data),
				test.options,
			)
			if !errors.Is(err, test.expectedError) {
				t.Errorf("Verify error = %v, want %v", err, test.expectedError)
			}
		})
	}
}

func TestStandardImageVerifierHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := utils.NewStandardImageVerifier(0, 0, 0).Verify(
		ctx,
		bytes.NewReader(encodeImage(t, "png", 1, 1)),
		utils.VerificationOptions{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Verify error = %v, want context.Canceled", err)
	}
}

func encodeImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()

	source := image.NewRGBA(image.Rect(0, 0, width, height))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})

	var data bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&data, source, nil)
	case "png":
		err = png.Encode(&data, source)
	default:
		t.Fatalf("unsupported test format %q", format)
	}
	if err != nil {
		t.Fatalf("encode %s fixture: %v", format, err)
	}

	return data.Bytes()
}
