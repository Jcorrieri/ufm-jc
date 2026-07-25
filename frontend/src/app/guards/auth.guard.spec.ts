import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { describe, expect, it, vi } from 'vitest';

import { authGuard } from './auth.guard';
import { AuthService, CurrentUser } from '../services/auth.service';

describe('authGuard', () => {
  it('allows a user loaded by AuthService', async () => {
    const user = { id: 'user-1' } as CurrentUser;
    const loadUser = vi.fn().mockResolvedValue(user);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: AuthService, useValue: { loadUser } },
      ],
    });

    const result = await TestBed.runInInjectionContext(() => authGuard(null!, null!));

    expect(result).toBe(true);
    expect(loadUser).toHaveBeenCalledOnce();
  });

  it('redirects when AuthService reports no authenticated user', async () => {
    const loadUser = vi.fn().mockResolvedValue(null);
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: AuthService, useValue: { loadUser } },
      ],
    });

    const result = await TestBed.runInInjectionContext(() => authGuard(null!, null!));

    expect(result).toEqual(TestBed.inject(Router).createUrlTree(['/login']));
  });
});
