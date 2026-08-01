import { Component, OnDestroy, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { FormsModule } from '@angular/forms';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';
import { ImageApiError, ImageService } from '../../services/image.service';

interface ImagePreview {
  file: File;
  url: string;
}

interface CreatedListing {
  id: string;
}

class ListingRequestError extends Error {
  constructor(
    message: string,
    readonly status?: number,
    readonly stateUnknown = false,
  ) {
    super(message);
  }
}

@Component({
  selector: 'app-create-listing-page',
  imports: [
    CommonModule,
    FormsModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
  ],
  templateUrl: './create-listing-page.html',
  styleUrl: './create-listing-page.css',
})
export class CreateListingPage implements OnDestroy {
  title = '';
  description = '';
  price: number | null = null;
  images: ImagePreview[] = [];
  submitting = signal(false);
  errorMsg = signal('');
  submissionStatus = signal('');

  constructor(
    private router: Router,
    private imageService: ImageService,
  ) {}

  ngOnDestroy() {
    this.revokeImagePreviews();
  }

  onImagesSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    const files = input.files;
    if (!files) return;

    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      if (!['image/jpeg', 'image/png'].includes(file.type)) {
        this.errorMsg.set('Only JPEG and PNG images are allowed.');
        continue;
      }
      if (file.size > 5 * 1024 * 1024) {
        this.errorMsg.set('Each image must be under 5MB.');
        continue;
      }
      const url = URL.createObjectURL(file);
      this.images.push({ file, url });
    }
    // Reset the input so re-selecting the same file works
    input.value = '';
  }

  removeImage(index: number) {
    URL.revokeObjectURL(this.images[index].url);
    this.images.splice(index, 1);
  }

  goBack() {
    this.router.navigate(['/profile']);
  }

  async submitListing() {
    if (!this.title.trim()) {
      this.errorMsg.set('Title is required.');
      return;
    }
    if (!this.description.trim()) {
      this.errorMsg.set('Description is required.');
      return;
    }
    if (this.price === null || this.price < 0) {
      this.errorMsg.set('Please enter a valid price.');
      return;
    }

    this.submitting.set(true);
    this.errorMsg.set('');
    this.submissionStatus.set('Creating draft…');
    let draftID: string | undefined;

    try {
      const draft = await this.createDraft();
      draftID = draft.id;

      for (const [position, preview] of this.images.entries()) {
        this.submissionStatus.set(
          `Uploading photo ${position + 1} of ${this.images.length}…`,
        );
        await this.imageService.uploadImage(preview.file, {
          listingId: draft.id,
          position,
        });
      }

      this.submissionStatus.set('Publishing listing…');
      await this.publishDraft(draft.id);
      this.router.navigate(['/main']);
    } catch (error) {
      let stateUnknown =
        (error instanceof ListingRequestError && error.stateUnknown) ||
        (error instanceof ImageApiError && error.stateUnknown);
      if (draftID && !stateUnknown) {
        stateUnknown = !(await this.abortDraft(draftID));
      }
      this.errorMsg.set(
        stateUnknown
          ? 'Listing status is unknown. Check My Listings before retrying.'
          : error instanceof TypeError
          ? 'Unable to reach the server.'
          : error instanceof Error
            ? error.message
            : 'Unable to create listing.',
      );
    } finally {
      this.submitting.set(false);
      this.submissionStatus.set('');
    }
  }

  private async createDraft(): Promise<CreatedListing> {
    const response = await fetch('/api/listings', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: this.title.trim(),
        description: this.description.trim(),
        price: this.price,
      }),
    });
    return this.readListingResponse(response, 'Failed to create listing draft.');
  }

  private async publishDraft(listingID: string): Promise<void> {
    for (let attempt = 0; attempt < 2; attempt++) {
      try {
        const response = await fetch(`/api/listings/${listingID}/publish`, {
          method: 'POST',
          credentials: 'include',
        });
        if (response.ok) {
          return;
        }
        if (response.status < 500) {
          throw new ListingRequestError(
            await this.errorMessage(response, 'Failed to publish listing.'),
            response.status,
          );
        }
      } catch (error) {
        if (error instanceof ListingRequestError && !error.stateUnknown) {
          throw error;
        }
      }
    }
    throw new ListingRequestError(
      'Publication status is unknown.',
      undefined,
      true,
    );
  }

  private async abortDraft(listingID: string): Promise<boolean> {
    try {
      const response = await fetch(`/api/listing-drafts/${listingID}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      return response.ok;
    } catch {
      return false;
    }
  }

  private async readListingResponse(
    response: Response,
    fallback: string,
  ): Promise<CreatedListing> {
    if (!response.ok) {
      throw new Error(await this.errorMessage(response, fallback));
    }
    return response.json() as Promise<CreatedListing>;
  }

  private async errorMessage(response: Response, fallback: string): Promise<string> {
    const body = await response.json().catch(() => null);
    return body?.error || fallback;
  }

  private revokeImagePreviews() {
    this.images.forEach((image) => URL.revokeObjectURL(image.url));
  }
}
