import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { AuthService } from '../services/auth.service';

export const authGuard: CanActivateFn = async () => {
  const router = inject(Router);
  const authService = inject(AuthService);

  try {
    if (await authService.loadUser()) {
      return true;
    }
  } catch {
    // network error — treat as unauthenticated
  }

  return router.createUrlTree(['/login']);
};
