import { Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { ImageService } from '../../services/image.service';
import { CreateListingPage } from './create-listing-page';

function validPage(
  imageService: Pick<ImageService, 'uploadImage' | 'remove'>,
  navigate = vi.fn(),
) {
  const page = new CreateListingPage(
    { navigate } as unknown as Router,
    imageService as ImageService,
  );
  page.title = 'Desk';
  page.description = 'A sturdy desk';
  page.price = 25;
  return { page, navigate };
}

describe('CreateListingPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('creates a draft, uploads images, and publishes in order', async () => {
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(Response.json({ id: 'listing-1' }, { status: 201 }))
      .mockResolvedValueOnce(Response.json({ id: 'listing-1', status: 'available' }));
    const uploadImage = vi
      .fn()
      .mockResolvedValueOnce({ id: 'image-1', status: 'ready' })
      .mockResolvedValueOnce({ id: 'image-2', status: 'ready' });
    const { page, navigate } = validPage({
      uploadImage,
      remove: vi.fn(),
    });
    page.images = [
      {
        file: new File(['one'], 'one.png', { type: 'image/png' }),
        url: 'blob:one',
      },
      {
        file: new File(['two'], 'two.jpg', { type: 'image/jpeg' }),
        url: 'blob:two',
      },
    ];

    await page.submitListing();

    expect(uploadImage).toHaveBeenNthCalledWith(
      1,
      page.images[0].file,
      { listingId: 'listing-1', position: 0 },
    );
    expect(uploadImage).toHaveBeenNthCalledWith(
      2,
      page.images[1].file,
      { listingId: 'listing-1', position: 1 },
    );
    expect(fetchSpy).toHaveBeenNthCalledWith(
      2,
      '/api/listings/listing-1/publish',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(navigate).toHaveBeenCalledWith(['/main']);
  });

  it('cleans up the draft when object storage is unavailable', async () => {
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(Response.json({ id: 'listing-1' }, { status: 201 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));
    const { page, navigate } = validPage({
      uploadImage: vi.fn().mockRejectedValue(new Error('Object storage unavailable')),
      remove: vi.fn(),
    });
    page.images = [
      {
        file: new File(['one'], 'one.png', { type: 'image/png' }),
        url: 'blob:one',
      },
    ];

    await page.submitListing();

    expect(page.errorMsg()).toBe('Object storage unavailable');
    expect(fetchSpy).toHaveBeenNthCalledWith(
      2,
      '/api/listing-drafts/listing-1',
      expect.objectContaining({ method: 'DELETE' }),
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it('retries an ambiguous publication without deleting the draft', async () => {
    const fetchSpy = vi
      .spyOn(window, 'fetch')
      .mockResolvedValueOnce(Response.json({ id: 'listing-1' }, { status: 201 }))
      .mockRejectedValueOnce(new TypeError('response lost'))
      .mockResolvedValueOnce(Response.json({ id: 'listing-1', status: 'available' }));
    const { page, navigate } = validPage({
      uploadImage: vi.fn(),
      remove: vi.fn(),
    });

    await page.submitListing();

    expect(navigate).toHaveBeenCalledWith(['/main']);
    expect(fetchSpy).toHaveBeenCalledTimes(3);
    expect(
      fetchSpy.mock.calls.some(([url]) => url === '/api/listing-drafts/listing-1'),
    ).toBe(false);
  });
});
