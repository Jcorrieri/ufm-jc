import { Component, ElementRef, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { ChatWidgetService } from '../../services/chat-widget.service';
import { ChatService, Conversation } from '../../services/chat.service';
import { AuthService } from '../../services/auth.service';
import { ChatPanel } from '../chat-panel/chat-panel';

@Component({
  selector: 'app-chat-widget',
  standalone: true,
  imports: [CommonModule, MatIconModule, MatButtonModule, ChatPanel],
  templateUrl: './chat-widget.html',
  styleUrl: './chat-widget.css',
})
export class ChatWidget {
  constructor(
    public widget: ChatWidgetService,
    private chatService: ChatService,
    private authService: AuthService,
    private el: ElementRef,
  ) {}

  get conversations() {
    return this.chatService.conversations;
  }

  get loading() {
    return this.chatService.conversationsLoading;
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent) {
    if (this.widget.state() !== 'closed' && !this.el.nativeElement.contains(event.target)) {
      this.widget.close();
    }
  }

  async toggle() {
    this.widget.toggle();
    if (this.widget.state() === 'list') {
      await this.refreshConversations();
    }
  }

  async refreshConversations() {
    if (!this.authService.currentUser()) return;
    try {
      await this.chatService.refreshConversations();
    } catch {
      // Preserve the last successful list if refreshing fails.
    }
  }

  selectConversation(convo: Conversation) {
    this.widget.openChat(convo);
  }

  backToList() {
    this.chatService.disconnect();
    this.chatService.clearHandlers();
    this.widget.backToList();
    this.refreshConversations();
  }

  getOtherName(convo: Conversation): string {
    const user = this.authService.currentUser();
    if (!user) return '';
    return user.id === convo.seller_id ? convo.buyer_name : convo.seller_name;
  }
}
