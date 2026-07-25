import { afterEach, describe, expect, it, vi } from 'vitest';

import { AuthService, CurrentUser } from './auth.service';

const apiUser = {
  id: 'user-1',
  first_name: 'Albert',
  last_name: 'Gator',
  email: 'albert@ufl.edu',
  image_id: null,
  created_at: '2026-07-24T12:00:00Z',
};

const currentUser: CurrentUser = {
  id: 'user-1',
  firstName: 'Albert',
  lastName: 'Gator',
  email: 'albert@ufl.edu',
  image_id: null,
  createdAt: '2026-07-24T12:00:00Z',
};

describe('AuthService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('reuses a loaded user without another request', async () => {
    const fetchSpy = vi.spyOn(window, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(apiUser), { status: 200 }),
    );
    const authService = new AuthService();

    expect(await authService.loadUser()).toEqual(currentUser);
    expect(await authService.loadUser()).toEqual(currentUser);
    expect(fetchSpy).toHaveBeenCalledOnce();
  });

  it('shares an in-flight current-user request', async () => {
    let resolveResponse!: (response: Response) => void;
    const responsePromise = new Promise<Response>(resolve => {
      resolveResponse = resolve;
    });
    const fetchSpy = vi.spyOn(window, 'fetch').mockReturnValue(responsePromise);
    const authService = new AuthService();

    const firstLoad = authService.loadUser();
    const secondLoad = authService.loadUser();
    resolveResponse(new Response(JSON.stringify(apiUser), { status: 200 }));

    expect(await Promise.all([firstLoad, secondLoad])).toEqual([currentUser, currentUser]);
    expect(fetchSpy).toHaveBeenCalledOnce();
  });

  it('clears and caches an unauthenticated result', async () => {
    const fetchSpy = vi.spyOn(window, 'fetch').mockResolvedValue(
      new Response(null, { status: 401 }),
    );
    const authService = new AuthService();

    expect(await authService.loadUser()).toBeNull();
    expect(await authService.loadUser()).toBeNull();
    expect(authService.currentUser()).toBeNull();
    expect(fetchSpy).toHaveBeenCalledOnce();
  });

  it('retries after a transient current-user failure', async () => {
    const fetchSpy = vi.spyOn(window, 'fetch')
      .mockResolvedValueOnce(new Response(null, { status: 500 }))
      .mockResolvedValueOnce(new Response(JSON.stringify(apiUser), { status: 200 }));
    const authService = new AuthService();

    await expect(authService.loadUser()).rejects.toThrow('Failed to load current user');
    await expect(authService.loadUser()).resolves.toEqual(currentUser);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });
});
