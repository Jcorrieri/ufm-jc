import { Component, OnInit, ChangeDetectorRef, signal } from '@angular/core';
import { CommonModule, CurrencyPipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatButtonModule } from '@angular/material/button';

interface MyListing {
  id: string;
  title: string;
  description: string;
  price: number;
  status: 'draft' | 'available' | 'sold';
  image_count: number;
  first_image_id: string | null;
  seller_name: string;
  created_at: string;
}

interface EditState {
  title: string;
  description: string;
  price: number;
}

@Component({
  selector: 'app-my-listings-page',
  imports: [
    CommonModule,
    FormsModule,
    MatIconModule,
    MatFormFieldModule,
    MatInputModule,
    MatButtonModule,
    CurrencyPipe,
  ],
  templateUrl: './my-listings-page.html',
  styleUrl: './my-listings-page.css',
})
export class MyListingsPage implements OnInit {
  listings: MyListing[] = [];
  editingId: string | null = null;
  editState: EditState = { title: '', description: '', price: 0 };
  saving = signal(false);
  errorMsg = signal('');

  constructor(
    private router: Router,
    private cdr: ChangeDetectorRef,
  ) {}

  async ngOnInit() {
    await this.loadListings();
  }

  async loadListings() {
    try {
      const res = await fetch('/api/listings/me', { credentials: 'include' });
      if (res.ok) {
        this.listings = (await res.json()) ?? [];
      }
    } catch {
      this.errorMsg.set('Failed to load your listings.');
    }
    this.cdr.detectChanges();
  }

  startEdit(listing: MyListing) {
    this.editingId = listing.id;
    this.editState = {
      title: listing.title,
      description: listing.description,
      price: listing.price,
    };
    this.errorMsg.set('');
  }

  cancelEdit() {
    this.editingId = null;
    this.editState = { title: '', description: '', price: 0 };
  }

  async saveEdit(listing: MyListing) {
    if (this.editState.price < 0) {
      this.errorMsg.set('Price must be positive.');
      return;
    }

    this.saving.set(true);
    this.errorMsg.set('');

    try {
      const res = await fetch(`/api/listings/${listing.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: this.editState.title,
          description: this.editState.description,
          price: this.editState.price,
        }),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        this.errorMsg.set(body.error || 'Failed to update listing.');
        return;
      }

      const updated = await res.json();
      const idx = this.listings.findIndex((l) => l.id === listing.id);
      if (idx !== -1) {
        this.listings[idx] = updated;
      }
      this.editingId = null;
      this.editState = { title: '', description: '', price: 0 };
    } catch {
      this.errorMsg.set('Unable to reach the server.');
    } finally {
      this.saving.set(false);
      this.cdr.detectChanges();
    }
  }

  async deleteListing(listing: MyListing) {
    if (!confirm(`Delete "${listing.title}"? This cannot be undone.`)) {
      return;
    }

    try {
      const res = await fetch(`/api/listings/${listing.id}`, {
        method: 'DELETE',
        credentials: 'include',
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        this.errorMsg.set(body.error || 'Failed to delete listing.');
        return;
      }

      this.listings = this.listings.filter((l) => l.id !== listing.id);
    } catch {
      this.errorMsg.set('Unable to reach the server.');
    }
    this.cdr.detectChanges();
  }

  goBack() {
    this.router.navigate(['/main']);
  }
}
