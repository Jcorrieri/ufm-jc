import { Component, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { MatButtonModule } from '@angular/material/button';
import { Router } from '@angular/router';
import { ChatService, Conversation } from '../../services/chat.service';
import { AuthService } from '../../services/auth.service';
import { ChatPanel } from '../../components/chat-panel/chat-panel';
import { AvatarDropdown } from '../../components/avatar-dropdown/avatar-dropdown';

@Component({
  selector: 'app-messages-page',
  standalone: true,
  imports: [CommonModule, MatIconModule, MatButtonModule, ChatPanel, AvatarDropdown],
  templateUrl: './messages-page.html',
  styleUrl: './messages-page.css',
})
export class MessagesPage implements OnInit {
  activeConversation = signal<Conversation | null>(null);

  constructor(
    private chatService: ChatService,
    private authService: AuthService,
    private router: Router,
  ) {}

  get conversations() {
    return this.chatService.conversations;
  }

  get loading() {
    return this.chatService.conversationsLoading;
  }

  async ngOnInit() {
    try {
      await this.chatService.refreshConversations();
    } catch {
      // Preserve the last successful list if refreshing fails.
    }
  }

  selectConversation(convo: Conversation) {
    this.chatService.disconnect();
    this.chatService.clearHandlers();
    this.activeConversation.set(convo);
  }

  getOtherName(convo: Conversation): string {
    const user = this.authService.currentUser();
    if (!user) return '';
    return user.id === convo.seller_id ? convo.buyer_name : convo.seller_name;
  }

  goBack() {
    this.router.navigate(['/main']);
  }
}
