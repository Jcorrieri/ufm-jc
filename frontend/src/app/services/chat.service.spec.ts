import { afterEach, describe, expect, it, vi } from 'vitest';

import { ChatService, Conversation, Message } from './chat.service';

class MockWebSocket {
  static readonly OPEN = 1;

  readonly url: string;
  readyState = 0;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: (() => void) | null = null;

  constructor(url: string) {
    this.url = url;
  }

  close() {
    this.readyState = 3;
  }

  send() {}
}

describe('ChatService', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
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
    expect(chatService.conversations()).toEqual([conversation]);
  });

  it('shares concurrent conversation refresh requests', async () => {
    const conversations = [{
      id: 'conversation-1',
      listing_id: 'listing-1',
      listing_title: 'Calculus Textbook',
      buyer_id: 'buyer-1',
      buyer_name: 'Buyer One',
      seller_id: 'seller-1',
      seller_name: 'Seller One',
      last_message: '',
      updated_at: '2026-07-23T12:00:00Z',
    }];
    const fetchSpy = vi.spyOn(window, 'fetch').mockResolvedValue(
      new Response(JSON.stringify(conversations), { status: 200 }),
    );
    const chatService = new ChatService();

    const firstRefresh = chatService.refreshConversations();
    const secondRefresh = chatService.refreshConversations();

    expect(await Promise.all([firstRefresh, secondRefresh])).toEqual([
      conversations,
      conversations,
    ]);
    expect(fetchSpy).toHaveBeenCalledOnce();
    expect(chatService.conversations()).toEqual(conversations);
  });

  it('creates WebSocket URLs from the current frontend origin', () => {
    const chatService = new ChatService();

    expect(
      chatService['createWebSocketUrl'](
        'conversation-1',
        'http://marketplace.example/messages',
      ),
    ).toBe('ws://marketplace.example/api/ws/chat/conversation-1');
    expect(
      chatService['createWebSocketUrl'](
        'conversation-1',
        'https://marketplace.example/messages',
      ),
    ).toBe('wss://marketplace.example/api/ws/chat/conversation-1');
  });

  it('updates conversation summaries before dispatching WebSocket messages', async () => {
    const sockets: MockWebSocket[] = [];
    class RecordingWebSocket extends MockWebSocket {
      constructor(url: string) {
        super(url);
        sockets.push(this);
      }
    }
    vi.stubGlobal('WebSocket', RecordingWebSocket);

    const message: Message = {
      id: 'message-1',
      conversation_id: 'conversation-1',
      sender_id: 'sender-1',
      sender_name: 'Sender One',
      content: 'Hello',
      created_at: '2026-07-24T12:00:00Z',
    };
    const olderConversation: Conversation = {
      id: 'conversation-2',
      listing_id: 'listing-2',
      listing_title: 'Desk',
      buyer_id: 'buyer-1',
      buyer_name: 'Buyer One',
      seller_id: 'seller-1',
      seller_name: 'Seller One',
      last_message: 'Older message',
      updated_at: '2026-07-23T10:00:00Z',
    };
    const activeConversation: Conversation = {
      ...olderConversation,
      id: 'conversation-1',
      listing_id: 'listing-1',
      last_message: 'Previous message',
      updated_at: '2026-07-23T09:00:00Z',
    };
    const fetchSpy = vi.spyOn(window, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify([olderConversation, activeConversation]),
        { status: 200 },
      ),
    );
    const handler = vi.fn();
    const chatService = new ChatService();
    await chatService.refreshConversations();
    chatService.onMessage(handler);

    chatService.connect('conversation-1');
    sockets[0].onmessage?.(
      new MessageEvent('message', { data: JSON.stringify(message) }),
    );

    expect(sockets).toHaveLength(1);
    expect(sockets[0].url).toContain('/api/ws/chat/conversation-1');
    expect(sockets[0].url).not.toContain('localhost:8080');
    expect(handler).toHaveBeenCalledWith(message);
    expect(chatService.conversations()[0]).toMatchObject({
      id: 'conversation-1',
      last_message: 'Hello',
      updated_at: '2026-07-24T12:00:00Z',
    });
    expect(fetchSpy).toHaveBeenCalledOnce();
  });
});
