import { afterEach, describe, expect, it, vi } from 'vitest';

import { ChatService, Conversation } from './chat.service';

describe('ChatService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('starts a conversation using only the listing ID', async () => {
    const conversation: Conversation = {
      id: 'conversation-1',
      listing_id: 'listing-1',
      listing_title: 'Calculus Textbook',
      buyer_id: 'buyer-1',
      buyer_name: 'Buyer One',
      seller_id: 'seller-1',
      seller_name: 'Seller One',
      last_message: '',
      updated_at: '2026-07-23T12:00:00Z',
    };
    const fetchSpy = vi.spyOn(window, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(conversation), { status: 200 }),
    );
    const chatService = new ChatService();

    const result = await chatService.startConversation('listing-1');

    expect(result).toEqual(conversation);
    expect(fetchSpy).toHaveBeenCalledOnce();
    expect(fetchSpy).toHaveBeenCalledWith('/api/conversations', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ listing_id: 'listing-1' }),
    });
  });
});
