import { Component, OnInit, signal } from '@angular/core';
import { RouterOutlet, Router, NavigationEnd } from '@angular/router';
import { ChatWidget } from './components/chat-widget/chat-widget';
import { ChatWidgetService } from './services/chat-widget.service';
import { filter } from 'rxjs/operators';

@Component({
  selector: 'app-root',
  imports: [RouterOutlet, ChatWidget],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App implements OnInit {
  private authRoutes = ['/login', '/sign-up', '/'];
  private widgetHiddenRoutes = new Set([
    '/',
    '/login',
    '/sign-up',
    '/forgot-password',
    '/reset-password',
    '/messages',
  ]);
  readonly showChatWidget = signal(false);

  constructor(
    private router: Router,
    private chatWidgetService: ChatWidgetService,
  ) {}

  ngOnInit() {
    // Apply theme on every route change
    this.router.events
      .pipe(filter(e => e instanceof NavigationEnd))
      .subscribe((e: any) => {
        this.applyTheme(e.urlAfterRedirects);
        this.updateChatWidgetVisibility(e.urlAfterRedirects);
      });

    // Apply on initial load
    this.applyTheme(window.location.pathname);
    this.updateChatWidgetVisibility(this.router.url);

    // Keep in sync if OS theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      this.applyTheme(this.router.url);
    });
  }

  private applyTheme(url: string) {
    const saved = (localStorage.getItem('theme') as 'light' | 'dark' | 'system') ?? 'system';
    const body = document.body;
    body.classList.remove('theme-light', 'theme-dark');

    // Never apply dark theme on auth pages
    const isAuthPage = this.authRoutes.some(r => url === r || url.startsWith(r + '?'));
    if (isAuthPage) {
      body.classList.add('theme-light');
      return;
    }

    if (saved === 'system') {
      const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      body.classList.add(prefersDark ? 'theme-dark' : 'theme-light');
    } else {
      body.classList.add(saved === 'dark' ? 'theme-dark' : 'theme-light');
    }
  }

  private updateChatWidgetVisibility(url: string) {
    const path = url.split(/[?#]/, 1)[0];
    const shouldShowChatWidget = !this.widgetHiddenRoutes.has(path);

    if (!shouldShowChatWidget) {
      this.chatWidgetService.close();
    }

    this.showChatWidget.set(shouldShowChatWidget);
  }
}
