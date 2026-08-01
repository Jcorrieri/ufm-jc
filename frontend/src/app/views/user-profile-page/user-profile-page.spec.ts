import { ChangeDetectorRef } from '@angular/core';
import { Router } from '@angular/router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuthService, CurrentUser } from '../../services/auth.service';
import { ImageService } from '../../services/image.service';
import { UserProfilePage } from './user-profile-page';

const currentUser: CurrentUser = {
  id: 'user-1',
  firstName: 'Albert',
  lastName: 'Gator',
  email: 'albert@ufl.edu',
  image_id: 'old-image',
};

function profilePage(uploadImage: ReturnType<typeof vi.fn>) {
  const setUser = vi.fn();
  const page = new UserProfilePage(
    {} as Router,
    {
      currentUser: () => currentUser,
      setUser,
    } as unknown as AuthService,
    { uploadImage } as unknown as ImageService,
    { detectChanges: vi.fn() } as unknown as ChangeDetectorRef,
  );
  page.user = currentUser;
  page.profileImageUrl = '/api/images/old-image';
  return { page, setUser };
}

function fileSelection(file: File): Event {
  return {
    target: { files: [file], value: 'selected' },
  } as unknown as Event;
}

describe('UserProfilePage image upload', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('updates the current user only after the image is verified', async () => {
    const uploadImage = vi.fn().mockResolvedValue({
      id: 'new-image',
      status: 'ready',
    });
    const { page, setUser } = profilePage(uploadImage);

    await page.onProfileImageSelected(
      fileSelection(new File(['image'], 'avatar.png', { type: 'image/png' })),
    );

    expect(page.user?.image_id).toBe('new-image');
    expect(page.profileImageUrl).toContain('/api/images/new-image');
    expect(setUser).toHaveBeenCalledWith(
      expect.objectContaining({ image_id: 'new-image' }),
    );
  });

  it('preserves the previous avatar when storage is unavailable', async () => {
    const uploadImage = vi
      .fn()
      .mockRejectedValue(new Error('Object storage unavailable'));
    const { page, setUser } = profilePage(uploadImage);

    await page.onProfileImageSelected(
      fileSelection(new File(['image'], 'avatar.png', { type: 'image/png' })),
    );

    expect(page.user?.image_id).toBe('old-image');
    expect(page.profileImageUrl).toBe('/api/images/old-image');
    expect(page.errorMsg()).toBe('Object storage unavailable');
    expect(setUser).not.toHaveBeenCalled();
  });

  it('ignores another selection while an upload is active', async () => {
    const uploadImage = vi.fn();
    const { page } = profilePage(uploadImage);
    page.saving.set(true);

    await page.onProfileImageSelected(
      fileSelection(new File(['image'], 'avatar.png', { type: 'image/png' })),
    );

    expect(uploadImage).not.toHaveBeenCalled();
  });
});
