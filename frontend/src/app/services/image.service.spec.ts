import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  BeginImageUploadResponse,
  ImageMetadata,
  ImageService,
  UploadAuthorization,
} from './image.service';

const file = new File(['image'], 'photo.png', { type: 'image/png' });
const image: ImageMetadata = {
  id: 'image-1',
  listing_id: 'listing-1',
  status: 'pending',
  position: 1,
  expected_size: file.size,
  expected_mime_type: file.type,
  upload_expires_at: '2026-07-26T12:00:00Z',
  created_at: '2026-07-26T11:45:00Z',
  updated_at: '2026-07-26T11:45:00Z',
};

function initiation(authorization: UploadAuthorization): BeginImageUploadResponse {
  return { image, authorization };
}

describe('ImageService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('coordinates initiation, a PUT upload, and completion', async () => {
    const readyImage = { ...image, status: 'ready' as const };
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(
        Response.json(
          initiation({
            url: 'https://objects.example/image-1',
            method: 'PUT',
            expires_at: '2026-07-26T12:00:00Z',
          }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(Response.json(readyImage));

    const result = await new ImageService().uploadImage(file, {
      listingId: 'listing-1',
      position: 1,
    });

    expect(result).toEqual(readyImage);
    expect(fetchSpy).toHaveBeenNthCalledWith(
      1,
      '/api/images/uploads',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
      }),
    );
    expect(JSON.parse(String(fetchSpy.mock.calls[0][1]?.body))).toEqual({
      listing_id: 'listing-1',
      expected_size: file.size,
      expected_mime_type: file.type,
      position: 1,
    });
    expect(fetchSpy).toHaveBeenNthCalledWith(
      2,
      'https://objects.example/image-1',
      expect.objectContaining({
        method: 'PUT',
        credentials: 'omit',
        body: file,
      }),
    );
    expect(fetchSpy).toHaveBeenNthCalledWith(
      3,
      '/api/images/image-1/complete',
      expect.objectContaining({ method: 'POST' }),
    );
  });

  it('uses authorization fields for a multipart POST upload', async () => {
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValue(new Response(null, { status: 204 }));
    const service = new ImageService();

    await service.uploadToObjectStore(file, {
      url: 'https://objects.example',
      method: 'POST',
      fields: { key: 'images/image-1', policy: 'signed-policy' },
      headers: { 'Content-Type': 'multipart/form-data', 'X-Upload': 'allowed' },
      expires_at: '2026-07-26T12:00:00Z',
    });

    const request = fetchSpy.mock.calls[0][1];
    expect(request?.credentials).toBe('omit');
    expect(request?.body).toBeInstanceOf(FormData);
    const body = request?.body as FormData;
    const headers = new Headers(request?.headers);
    expect(headers.has('Content-Type')).toBe(false);
    expect(headers.get('X-Upload')).toBe('allowed');
    expect(body.get('key')).toBe('images/image-1');
    expect(body.get('policy')).toBe('signed-policy');
    expect(body.get('file')).toBe(file);
  });

  it('removes initiated metadata when the object upload fails', async () => {
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(
        Response.json(
          initiation({
            url: 'https://objects.example/image-1',
            method: 'PUT',
            expires_at: '2026-07-26T12:00:00Z',
          }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(new Response(null, { status: 500 }))
      .mockResolvedValueOnce(new Response(null, { status: 202 }));

    await expect(new ImageService().uploadImage(file)).rejects.toThrow(
      'Failed to upload image data.',
    );
    expect(fetchSpy).toHaveBeenNthCalledWith(
      3,
      '/api/images/image-1',
      expect.objectContaining({ method: 'DELETE' }),
    );
  });

  it('reconciles a lost completion response without deleting the image', async () => {
    const readyImage = { ...image, status: 'ready' as const };
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(
        Response.json(
          initiation({
            url: 'https://objects.example/image-1',
            method: 'PUT',
            expires_at: '2026-07-26T12:00:00Z',
          }),
          { status: 201 },
        ),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockRejectedValueOnce(new TypeError('response lost'))
      .mockResolvedValueOnce(Response.json(readyImage));

    await expect(new ImageService().uploadImage(file)).resolves.toEqual(readyImage);
    expect(fetchSpy).toHaveBeenCalledTimes(4);
    expect(fetchSpy.mock.calls.some(([url]) => url === '/api/images/image-1')).toBe(false);
  });

  it('surfaces the unavailable object-store response from initiation', async () => {
    vi.spyOn(window, 'fetch').mockResolvedValue(
      Response.json({ error: 'Object storage unavailable' }, { status: 503 }),
    );

    await expect(new ImageService().uploadImage(file)).rejects.toThrow(
      'Object storage unavailable',
    );
  });
});
