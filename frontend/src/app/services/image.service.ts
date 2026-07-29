import { Injectable } from '@angular/core';

export type ImageStatus = 'pending' | 'verifying' | 'ready' | 'failed' | 'deleting';

export interface ImageMetadata {
  id: string;
  listing_id?: string;
  status: ImageStatus;
  position: number;
  expected_size: number;
  expected_mime_type: string;
  size?: number;
  mime_type?: string;
  width?: number;
  height?: number;
  checksum_sha256?: string;
  upload_expires_at: string;
  verification_started_at?: string;
  verified_at?: string;
  created_at: string;
  updated_at: string;
}

export interface UploadAuthorization {
  url: string;
  method: 'PUT' | 'POST';
  headers?: Record<string, string>;
  fields?: Record<string, string>;
  expires_at: string;
}

export interface BeginImageUploadResponse {
  image: ImageMetadata;
  authorization: UploadAuthorization;
}

export interface UploadImageOptions {
  listingId?: string;
  position?: number;
}

export class ImageApiError extends Error {
  constructor(
    message: string,
    readonly imageId?: string,
    readonly status?: number,
    readonly stateUnknown = false,
  ) {
    super(message);
  }
}

@Injectable({ providedIn: 'root' })
export class ImageService {
  async beginUpload(
    file: File,
    options: UploadImageOptions = {},
  ): Promise<BeginImageUploadResponse> {
    const response = await fetch('/api/images/uploads', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        listing_id: options.listingId,
        expected_size: file.size,
        expected_mime_type: file.type,
        position: options.position ?? 0,
      }),
    });
    return this.readJson<BeginImageUploadResponse>(
      response,
      'Failed to initialize image upload.',
    );
  }

  async uploadToObjectStore(
    file: File,
    authorization: UploadAuthorization,
  ): Promise<void> {
    const method = authorization.method.toUpperCase();
    let body: BodyInit;
    let headers: HeadersInit | undefined;

    if (method === 'POST') {
      const formData = new FormData();
      for (const [name, value] of Object.entries(authorization.fields ?? {})) {
        formData.append(name, value);
      }
      formData.append('file', file);
      body = formData;
      const uploadHeaders = new Headers(authorization.headers);
      uploadHeaders.delete('Content-Type');
      headers = uploadHeaders;
    } else if (method === 'PUT') {
      const uploadHeaders = new Headers(authorization.headers);
      if (!uploadHeaders.has('Content-Type')) {
        uploadHeaders.set('Content-Type', file.type);
      }
      body = file;
      headers = uploadHeaders;
    } else {
      throw new ImageApiError(`Unsupported object upload method: ${method}`);
    }

    const response = await fetch(authorization.url, {
      method,
      credentials: 'omit',
      headers,
      body,
    });
    if (!response.ok) {
      throw new ImageApiError('Failed to upload image data.');
    }
  }

  async completeUpload(imageId: string): Promise<ImageMetadata> {
    const response = await fetch(`/api/images/${imageId}/complete`, {
      method: 'POST',
      credentials: 'include',
    });
    return this.readJson<ImageMetadata>(response, 'Failed to verify image.');
  }

  async getStatus(imageId: string): Promise<ImageMetadata> {
    const response = await fetch(`/api/images/${imageId}/status`, {
      credentials: 'include',
    });
    return this.readJson<ImageMetadata>(response, 'Failed to load image status.');
  }

  async remove(imageId: string): Promise<void> {
    const response = await fetch(`/api/images/${imageId}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    if (!response.ok) {
      throw new ImageApiError(
        await this.errorMessage(response, 'Failed to remove image.'),
        imageId,
        response.status,
      );
    }
  }

  async uploadImage(
    file: File,
    options: UploadImageOptions = {},
  ): Promise<ImageMetadata> {
    let imageId: string | undefined;
    let objectUploaded = false;
    try {
      const initiation = await this.beginUpload(file, options);
      imageId = initiation.image.id;
      await this.uploadToObjectStore(file, initiation.authorization);
      objectUploaded = true;
      return await this.completeWithReconciliation(imageId);
    } catch (error) {
      if (imageId && !objectUploaded && !options.listingId) {
        await this.remove(imageId).catch(() => undefined);
      }
      if (error instanceof ImageApiError) {
        throw new ImageApiError(
          error.message,
          imageId,
          error.status,
          error.stateUnknown,
        );
      }
      const message =
        error instanceof TypeError
          ? 'Unable to reach the image service.'
          : error instanceof Error
            ? error.message
            : 'Image upload failed.';
      throw new ImageApiError(message, imageId);
    }
  }

  private async completeWithReconciliation(imageId: string): Promise<ImageMetadata> {
    try {
      return await this.completeUpload(imageId);
    } catch (completionError) {
      if (
        completionError instanceof ImageApiError &&
        completionError.status !== undefined &&
        completionError.status < 500 &&
        completionError.status !== 409
      ) {
        throw completionError;
      }

      try {
        const image = await this.getStatus(imageId);
        if (image.status === 'ready') {
          return image;
        }
        if (image.status === 'pending') {
          return await this.completeUpload(imageId);
        }
        if (image.status === 'failed' || image.status === 'deleting') {
          throw new ImageApiError('Image verification failed.', imageId, 422);
        }
      } catch (statusError) {
        if (
          statusError instanceof ImageApiError &&
          statusError.status !== undefined &&
          statusError.status < 500
        ) {
          throw statusError;
        }
      }
      throw new ImageApiError(
        'Image verification status is unknown. Check again later.',
        imageId,
        undefined,
        true,
      );
    }
  }

  private async readJson<T>(response: Response, fallback: string): Promise<T> {
    if (!response.ok) {
      throw new ImageApiError(
        await this.errorMessage(response, fallback),
        undefined,
        response.status,
      );
    }
    return response.json() as Promise<T>;
  }

  private async errorMessage(response: Response, fallback: string): Promise<string> {
    const body = await response.json().catch(() => null);
    return body?.error || fallback;
  }
}
