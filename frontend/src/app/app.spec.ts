import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';
import { ChatWidgetService } from './services/chat-widget.service';

describe('App', () => {
  beforeEach(async () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        addEventListener: vi.fn(),
      }),
    );
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    expect(app).toBeTruthy();
  });

  it('hides the chat widget on public and messages routes', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    fixture.detectChanges();
    const hiddenRoutes = [
      '/',
      '/login',
      '/sign-up',
      '/forgot-password',
      '/reset-password?token=reset-token',
      '/messages',
    ];

    for (const route of hiddenRoutes) {
      app['updateChatWidgetVisibility'](route);
      fixture.detectChanges();
      expect(fixture.nativeElement.querySelector('app-chat-widget')).toBeNull();
    }
  });

  it('shows the chat widget on marketplace routes', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    fixture.detectChanges();

    app['updateChatWidgetVisibility']('/main');
    fixture.detectChanges();

    expect(fixture.nativeElement.querySelector('app-chat-widget')).not.toBeNull();
  });

  it('closes the chat widget when navigating to a hidden route', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    const chatWidgetService = TestBed.inject(ChatWidgetService);
    chatWidgetService.open();

    app['updateChatWidgetVisibility']('/messages');

    expect(chatWidgetService.state()).toBe('closed');
    expect(chatWidgetService.activeConversation()).toBeNull();
  });
});
