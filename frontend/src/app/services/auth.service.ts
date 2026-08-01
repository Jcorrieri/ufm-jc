import { Injectable, signal } from '@angular/core';

export interface CurrentUser {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  image_id?: string | null;
  createdAt?: string; 
}

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly userSignal = signal<CurrentUser | null>(null);
  private userLoaded = false;
  private userLoadPromise: Promise<CurrentUser | null> | null = null;
  readonly currentUser = this.userSignal.asReadonly();

  async loadUser(): Promise<CurrentUser | null> {
    if (this.userLoaded) {
      return this.currentUser();
    }

    if (!this.userLoadPromise) {
      this.userLoadPromise = this.fetchUser().finally(() => {
        this.userLoadPromise = null;
      });
    }

    return this.userLoadPromise;
  }

  setUser(user: CurrentUser): void {
    this.userSignal.set({ ...user });
    this.userLoaded = true;
  }

  clearUser(): void {
    this.userSignal.set(null);
    this.userLoaded = true;
  }

  async logout(): Promise<void> {
    try {
      await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
    } catch (e) {
      console.error('logout request failed', e);
    }
    this.clearUser();
  }

  private async fetchUser(): Promise<CurrentUser | null> {
    const response = await fetch('/api/users/me', { credentials: 'include' });
    if (response.status === 401 || response.status === 403) {
      this.clearUser();
      return null;
    }
    if (!response.ok) {
      throw new Error('Failed to load current user');
    }

    const data = await response.json();
    const user = {
      id: data.id,
      firstName: data.first_name,
      lastName: data.last_name,
      email: data.email,
      image_id: data.image_id,
      createdAt: data.created_at,
    };
    this.setUser(user);
    return user;
  }
}
