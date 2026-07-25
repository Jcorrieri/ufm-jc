import { ChangeDetectorRef } from '@angular/core';
import { describe, expect, it, vi } from 'vitest';

import { ChatPanel } from './chat-panel';
import { AuthService } from '../../services/auth.service';
import { ChatService } from '../../services/chat.service';

function createChatPanel() {
  const chatService = {
    sendMessage: vi.fn(),
  } as unknown as ChatService;
  const chatPanel = new ChatPanel(
    chatService,
    {} as AuthService,
    {} as ChangeDetectorRef,
  );

  return { chatPanel, chatService };
}

describe('ChatPanel', () => {
  it('sends multiline messages within the character limit', () => {
    const { chatPanel, chatService } = createChatPanel();
    chatPanel.newMessage = 'First line\nSecond line';

    chatPanel.send();

    expect(chatService.sendMessage).toHaveBeenCalledWith('First line\nSecond line');
    expect(chatPanel.newMessage).toBe('');
  });

  it('does not send messages over 750 characters', () => {
    const { chatPanel, chatService } = createChatPanel();
    chatPanel.newMessage = 'a'.repeat(751);

    chatPanel.send();

    expect(chatService.sendMessage).not.toHaveBeenCalled();
    expect(chatPanel.newMessage).toHaveLength(751);
  });

  it('uses Ctrl or Command plus Enter to send while leaving Enter for newlines', () => {
    const { chatPanel, chatService } = createChatPanel();
    const plainEnter = {
      key: 'Enter',
      ctrlKey: false,
      metaKey: false,
      preventDefault: vi.fn(),
    } as unknown as KeyboardEvent;
    chatPanel.newMessage = 'Draft';

    chatPanel.onKeyDown(plainEnter);

    expect(plainEnter.preventDefault).not.toHaveBeenCalled();
    expect(chatService.sendMessage).not.toHaveBeenCalled();

    const commandEnter = {
      key: 'Enter',
      ctrlKey: false,
      metaKey: true,
      preventDefault: vi.fn(),
    } as unknown as KeyboardEvent;
    chatPanel.onKeyDown(commandEnter);

    expect(commandEnter.preventDefault).toHaveBeenCalledOnce();
    expect(chatService.sendMessage).toHaveBeenCalledWith('Draft');
  });
});
