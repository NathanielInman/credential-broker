// The request window: one fixed-size GTK window holding a WebKit view of
// the embedded page. The page talks to Go through the "broker" script
// message handler; Go talks to the page by evaluating broker.receive().

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

#include "window.h"
#include "_cgo_export.h"

static GtkWidget *win;
static WebKitWebView *view;
static int win_width;

static void on_message(WebKitUserContentManager *m, WebKitJavascriptResult *r, gpointer data) {
	JSCValue *v = webkit_javascript_result_get_js_value(r);
	if (!jsc_value_is_string(v)) {
		return;
	}
	char *s = jsc_value_to_string(v);
	goMessage(s);
	g_free(s);
}

// Closing the window is a deny.
static gboolean on_delete(GtkWidget *w, GdkEvent *e, gpointer data) {
	goClosed();
	return FALSE;
}

static void on_destroy(GtkWidget *w, gpointer data) {
	gtk_main_quit();
}

// The page never goes anywhere: only the initial load of the embedded page
// is allowed, and no link or script can open another.
static gboolean on_policy(WebKitWebView *v, WebKitPolicyDecision *d, WebKitPolicyDecisionType type, gpointer data) {
	if (type == WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION) {
		WebKitNavigationAction *a = webkit_navigation_policy_decision_get_navigation_action(WEBKIT_NAVIGATION_POLICY_DECISION(d));
		const char *uri = webkit_uri_request_get_uri(webkit_navigation_action_get_request(a));
		if (g_strcmp0(uri, "about:blank") == 0) {
			webkit_policy_decision_use(d);
			return TRUE;
		}
	}
	if (type == WEBKIT_POLICY_DECISION_TYPE_RESPONSE) {
		webkit_policy_decision_use(d);
		return TRUE;
	}
	webkit_policy_decision_ignore(d);
	return TRUE;
}

static gboolean on_context_menu(WebKitWebView *v, WebKitContextMenu *m, GdkEvent *e, WebKitHitTestResult *h, gpointer data) {
	return TRUE;
}

// fix_size pins the window to one size with equal min and max hints. Not
// gtk_window_set_resizable(FALSE): on Wayland that window follows its size
// request up but never back down, so a short request kept the initial
// height as dead space under the buttons.
static void fix_size(int width, int height) {
	GdkGeometry g = {
		.min_width = width, .min_height = height,
		.max_width = width, .max_height = height,
	};
	gtk_window_set_geometry_hints(GTK_WINDOW(win), NULL, &g, GDK_HINT_MIN_SIZE | GDK_HINT_MAX_SIZE);
}

void window_create(const char *html, int width, int height) {
	// The Wayland app id, for compositor rules: Hyprland's class:^(broker)$.
	g_set_prgname("broker");
	gtk_init(NULL, NULL);
	win_width = width;

	win = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_title(GTK_WINDOW(win), "broker request");
	// A fixed size, which tiling compositors take as a dialog to float.
	fix_size(width, height);
	gtk_window_set_keep_above(GTK_WINDOW(win), TRUE);
	gtk_window_set_position(GTK_WINDOW(win), GTK_WIN_POS_CENTER);
	gtk_window_set_type_hint(GTK_WINDOW(win), GDK_WINDOW_TYPE_HINT_DIALOG);

	WebKitUserContentManager *m = webkit_user_content_manager_new();
	g_signal_connect(m, "script-message-received::broker", G_CALLBACK(on_message), NULL);
	webkit_user_content_manager_register_script_message_handler(m, "broker");
	view = WEBKIT_WEB_VIEW(webkit_web_view_new_with_user_content_manager(m));

	WebKitSettings *s = webkit_web_view_get_settings(view);
	webkit_settings_set_enable_developer_extras(s, FALSE);
	webkit_settings_set_enable_write_console_messages_to_stdout(s, FALSE);
	webkit_settings_set_enable_page_cache(s, FALSE);
	webkit_settings_set_enable_html5_local_storage(s, FALSE);
	webkit_settings_set_enable_html5_database(s, FALSE);
	webkit_settings_set_javascript_can_open_windows_automatically(s, FALSE);

	GdkRGBA bg = {0.106, 0.110, 0.129, 1.0};
	webkit_web_view_set_background_color(view, &bg);
	gtk_widget_set_size_request(GTK_WIDGET(view), width, height);
	gtk_container_add(GTK_CONTAINER(win), GTK_WIDGET(view));

	g_signal_connect(view, "decide-policy", G_CALLBACK(on_policy), NULL);
	g_signal_connect(view, "context-menu", G_CALLBACK(on_context_menu), NULL);
	g_signal_connect(win, "delete-event", G_CALLBACK(on_delete), NULL);
	g_signal_connect(win, "destroy", G_CALLBACK(on_destroy), NULL);

	webkit_web_view_load_html(view, html, NULL);
	gtk_widget_show_all(win);
	gtk_window_present(GTK_WINDOW(win));
}

void window_run(void) {
	gtk_main();
}

// Called from the page's message handler, so already on the GTK thread.
void window_resize(int height) {
	if (height < 120) {
		height = 120;
	}
	if (height > 900) {
		height = 900;
	}
	gtk_widget_set_size_request(GTK_WIDGET(view), win_width, height);
	fix_size(win_width, height);
	gtk_window_resize(GTK_WINDOW(win), win_width, height);
}

static gboolean eval_idle(gpointer js) {
	webkit_web_view_evaluate_javascript(view, js, -1, NULL, NULL, NULL, NULL, NULL);
	g_free(js);
	return G_SOURCE_REMOVE;
}

// Safe from any thread: the script runs on the GTK thread.
void window_eval(const char *js) {
	g_idle_add(eval_idle, g_strdup(js));
}

static gboolean quit_idle(gpointer data) {
	gtk_main_quit();
	return G_SOURCE_REMOVE;
}

// Safe from any thread.
void window_quit(void) {
	g_idle_add(quit_idle, NULL);
}
