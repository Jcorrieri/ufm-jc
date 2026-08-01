import { Injectable, signal } from '@angular/core';

export interface Conversation {
  id: string;
  listing_id: string;
  listing_title: string;
  buyer_id: string;
  buyer_name: string;
  seller_id: string;
  seller_name: string;
  last_message: string;
  updated_at: string;
}

export interface Message {
  id: string;
  conversation_id: string;
  sender_id: string;
  sender_name: string;
  content: string;
  created_at: string;
}

@Injectable({
  providedIn: 'root'
})
export class ChatService {
  private socket: WebSocket | null = null;
  private messageHandlers: ((msg: Message) => void)[] = [];
  private conversationLoadPromise: Promise<Conversation[]> | null = null;
  private readonly conversationsSignal = signal<Conversation[]>([]);
  private readonly conversationsLoadingSignal = signal(false);
  readonly conversations = this.conversationsSignal.asReadonly();
  readonly conversationsLoading = this.conversationsLoadingSignal.asReadonly();

  async startConversation(listingId: string): Promise<Conversation> {
    const res = await fetch('/api/conversations', {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ listing_id: listingId }),
    });
    if (!res.ok) throw new Error('Failed to start conversation');
    const conversation = await res.json();
    this.upsertConversation(conversation);
    return conversation;
  }

  async refreshConversations(): Promise<Conversation[]> {
    if (!this.conversationLoadPromise) {
      this.conversationsLoadingSignal.set(true);
      this.conversationLoadPromise = this.fetchConversations().finally(() => {
        this.conversationLoadPromise = null;
        this.conversationsLoadingSignal.set(false);
      });
    }

    return this.conversationLoadPromise;
  }

  async getMessages(conversationId: string): Promise<Message[]> {
    const res = await fetch(`/api/conversations/${conversationId}/messages`, {
      credentials: 'include',
    });
    if (!res.ok) throw new Error('Failed to fetch messages');
    const data = await res.json();
    return data ?? [];   // ← handle null from backend
  }

  connect(conversationId: string): void {
    this.disconnect();
    const url = this.createWebSocketUrl(conversationId);
    this.socket = new WebSocket(url);
    this.socket.onmessage = (event) => {
      try {
        const msg: Message = JSON.parse(event.data);
        this.updateConversationFromMessage(msg);
        this.messageHandlers.forEach(handler => handler(msg));
      } catch {
        console.error('Failed to parse incoming message', event.data);
      }
    };
    this.socket.onerror = (err) => console.error('WebSocket error', err);
    this.socket.onclose = () => console.log('WebSocket closed');
  }

  sendMessage(content: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(content);
    } else {
      console.warn('WebSocket not open');
    }
  }

  onMessage(handler: (msg: Message) => void): void {
    this.messageHandlers.push(handler);
  }

  clearHandlers(): void {
    this.messageHandlers = [];
  }

  disconnect(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }

  private createWebSocketUrl(
    conversationId: string,
    pageUrl = window.location.href,
  ): string {
    const url = new URL(`/api/ws/chat/${encodeURIComponent(conversationId)}`, pageUrl);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return url.toString();
  }

  private async fetchConversations(): Promise<Conversation[]> {
    const response = await fetch('/api/conversations', { credentials: 'include' });
    if (!response.ok) {
      throw new Error('Failed to fetch conversations');
    }

    const conversations = (await response.json()) ?? [];
    this.conversationsSignal.set(conversations);
    return conversations;
  }

  private upsertConversation(conversation: Conversation): void {
    const conversations = this.conversations();
    const existingIndex = conversations.findIndex(item => item.id === conversation.id);
    if (existingIndex === -1) {
      this.conversationsSignal.set([conversation, ...conversations]);
      return;
    }

    const existing = conversations[existingIndex];
    const updated = {
      ...existing,
      ...conversation,
      last_message: conversation.last_message || existing.last_message,
    };
    this.conversationsSignal.set([
      updated,
      ...conversations.filter(item => item.id !== conversation.id),
    ]);
  }

  private updateConversationFromMessage(message: Message): void {
    const conversation = this.conversations().find(
      item => item.id === message.conversation_id,
    );
    if (!conversation) {
      return;
    }

    this.conversationsSignal.set([
      {
        ...conversation,
        last_message: message.content,
        updated_at: message.created_at,
      },
      ...this.conversations().filter(item => item.id !== message.conversation_id),
    ]);
  }
}
