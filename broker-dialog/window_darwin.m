// The request window on macOS: one floating NSWindow holding a WKWebView of
// the embedded page. The page talks to Go through the "broker" script
// message handler; Go talks to the page by evaluating broker.receive().
// Manual reference counting: everything here lives as long as the process.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

#include "window.h"
#include "_cgo_export.h"

static NSWindow *win;
static WKWebView *view;
static CGFloat winWidth;

// stopApp ends [NSApp run]. stop: takes effect after the next event, so
// post one.
static void stopApp(void) {
	[NSApp stop:nil];
	NSEvent *e = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
	                                location:NSZeroPoint
	                           modifierFlags:0
	                               timestamp:0
	                            windowNumber:0
	                                 context:nil
	                                 subtype:0
	                                   data1:0
	                                   data2:0];
	[NSApp postEvent:e atStart:YES];
}

@interface BrokerDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandler, WKNavigationDelegate>
@end

@implementation BrokerDelegate

- (void)userContentController:(WKUserContentController *)controller didReceiveScriptMessage:(WKScriptMessage *)message {
	if ([message.body isKindOfClass:[NSString class]]) {
		goMessage((char *)[(NSString *)message.body UTF8String]);
	}
}

// Closing the window is a deny.
- (void)windowWillClose:(NSNotification *)notification {
	goClosed();
	stopApp();
}

// The page never goes anywhere: only the initial load of the embedded page
// is allowed, and no link or script can open another.
- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationAction:(WKNavigationAction *)action
                    decisionHandler:(void (^)(WKNavigationActionPolicy))handler {
	if ([[action.request.URL absoluteString] isEqualToString:@"about:blank"]) {
		handler(WKNavigationActionPolicyAllow);
	} else {
		handler(WKNavigationActionPolicyCancel);
	}
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)app {
	return NO;
}

@end

// editMenu gives the page Cut, Copy, Paste, and Select All, so a
// passphrase can be pasted from a password manager.
static NSMenu *editMenu(void) {
	NSMenu *bar = [[NSMenu alloc] init];
	NSMenuItem *appItem = [[NSMenuItem alloc] init];
	NSMenu *app = [[NSMenu alloc] init];
	[app addItemWithTitle:@"Close" action:@selector(performClose:) keyEquivalent:@"w"];
	[appItem setSubmenu:app];
	[bar addItem:appItem];
	NSMenuItem *editItem = [[NSMenuItem alloc] init];
	NSMenu *edit = [[NSMenu alloc] initWithTitle:@"Edit"];
	[edit addItemWithTitle:@"Cut" action:@selector(cut:) keyEquivalent:@"x"];
	[edit addItemWithTitle:@"Copy" action:@selector(copy:) keyEquivalent:@"c"];
	[edit addItemWithTitle:@"Paste" action:@selector(paste:) keyEquivalent:@"v"];
	[edit addItemWithTitle:@"Select All" action:@selector(selectAll:) keyEquivalent:@"a"];
	[editItem setSubmenu:edit];
	[bar addItem:editItem];
	return bar;
}

void window_create(const char *html, int width, int height) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		// No Dock icon or menu bar of its own, but it can take focus.
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		BrokerDelegate *delegate = [[BrokerDelegate alloc] init];
		[NSApp setDelegate:delegate];
		[NSApp setMainMenu:editMenu()];
		winWidth = width;

		NSRect frame = NSMakeRect(0, 0, width, height);
		win = [[NSWindow alloc] initWithContentRect:frame
		                                  styleMask:(NSWindowStyleMaskTitled | NSWindowStyleMaskClosable)
		                                    backing:NSBackingStoreBuffered
		                                      defer:NO];
		[win setTitle:@"broker request"];
		[win setReleasedWhenClosed:NO];
		[win setLevel:NSFloatingWindowLevel];
		[win setBackgroundColor:[NSColor colorWithSRGBRed:0.106 green:0.110 blue:0.129 alpha:1.0]];
		[win setDelegate:delegate];

		WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
		[config.userContentController addScriptMessageHandler:delegate name:@"broker"];
		view = [[WKWebView alloc] initWithFrame:frame configuration:config];
		view.navigationDelegate = delegate;
		[win setContentView:view];
		[view loadHTMLString:[NSString stringWithUTF8String:html] baseURL:nil];

		[win center];
		[win makeKeyAndOrderFront:nil];
		[NSApp activateIgnoringOtherApps:YES];
	}
}

void window_run(void) {
	[NSApp run];
}

// Called from the page's message handler, so already on the main thread.
// The window grows or shrinks from its top edge.
void window_resize(int height) {
	if (height < 120) {
		height = 120;
	}
	if (height > 900) {
		height = 900;
	}
	NSRect frame = [win frame];
	NSRect content = NSMakeRect(0, 0, winWidth, height);
	NSRect next = [win frameRectForContentRect:content];
	next.origin.x = frame.origin.x;
	next.origin.y = NSMaxY(frame) - next.size.height;
	[win setFrame:next display:YES animate:NO];
}

// Safe from any thread: the script runs on the main thread.
void window_eval(const char *js) {
	@autoreleasepool {
		NSString *script = [[NSString alloc] initWithUTF8String:js];
		dispatch_async(dispatch_get_main_queue(), ^{
			[view evaluateJavaScript:script completionHandler:nil];
			[script release];
		});
	}
}

// Safe from any thread. It ends the app without closing the window, which
// would count as a deny.
void window_quit(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		stopApp();
	});
}
