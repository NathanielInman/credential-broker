// The Signal Box panel: a transparent layer-shell surface on the overlay
// layer that covers one monitor (less what other surfaces reserve), holding
// a WebKit view of the embedded page. The page draws the panel where the
// bar icon is and animates it open and closed; this file only maps and
// unmaps the surface on the right monitor. The page talks to Go through the
// "board" script message handler; Go talks to the page by evaluating
// board.receive().

#include <gtk/gtk.h>
#include <gtk-layer-shell.h>
#include <webkit2/webkit2.h>

#include "window.h"
#include "_cgo_export.h"

static GtkWidget *win;
static WebKitWebView *view;

static void on_message(WebKitUserContentManager *m, WebKitJavascriptResult *r, gpointer data) {
	JSCValue *v = webkit_javascript_result_get_js_value(r);
	if (!jsc_value_is_string(v)) {
		return;
	}
	char *s = jsc_value_to_string(v);
	goMessage(s);
	g_free(s);
}

static gboolean on_delete(GtkWidget *w, GdkEvent *e, gpointer data) {
	gtk_widget_hide(win);
	goHidden();
	return TRUE;
}

// The page never goes anywhere: only the initial load of the embedded page.
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

void window_create(const char *html) {
	g_set_prgname("broker-board");
	gtk_init(NULL, NULL);

	win = gtk_window_new(GTK_WINDOW_TOPLEVEL);
	gtk_window_set_title(GTK_WINDOW(win), "Signal Box");
	gtk_layer_init_for_window(GTK_WINDOW(win));
	gtk_layer_set_namespace(GTK_WINDOW(win), "broker-board");
	gtk_layer_set_layer(GTK_WINDOW(win), GTK_LAYER_SHELL_LAYER_OVERLAY);
	gtk_layer_set_anchor(GTK_WINDOW(win), GTK_LAYER_SHELL_EDGE_TOP, TRUE);
	gtk_layer_set_anchor(GTK_WINDOW(win), GTK_LAYER_SHELL_EDGE_BOTTOM, TRUE);
	gtk_layer_set_anchor(GTK_WINDOW(win), GTK_LAYER_SHELL_EDGE_LEFT, TRUE);
	gtk_layer_set_anchor(GTK_WINDOW(win), GTK_LAYER_SHELL_EDGE_RIGHT, TRUE);
	// Zero: the surface keeps clear of the bar and other exclusive zones,
	// and reserves nothing itself.
	gtk_layer_set_exclusive_zone(GTK_WINDOW(win), 0);
	gtk_layer_set_keyboard_mode(GTK_WINDOW(win), GTK_LAYER_SHELL_KEYBOARD_MODE_EXCLUSIVE);

	// A transparent surface: everything outside the panel shows through,
	// and a click there closes it.
	GdkScreen *screen = gtk_widget_get_screen(win);
	GdkVisual *visual = gdk_screen_get_rgba_visual(screen);
	if (visual != NULL) {
		gtk_widget_set_visual(win, visual);
	}
	gtk_widget_set_app_paintable(win, TRUE);

	WebKitUserContentManager *m = webkit_user_content_manager_new();
	g_signal_connect(m, "script-message-received::board", G_CALLBACK(on_message), NULL);
	webkit_user_content_manager_register_script_message_handler(m, "board");
	view = WEBKIT_WEB_VIEW(webkit_web_view_new_with_user_content_manager(m));

	WebKitSettings *s = webkit_web_view_get_settings(view);
	webkit_settings_set_enable_developer_extras(s, g_getenv("BROKER_BOARD_DEVTOOLS") != NULL);
	webkit_settings_set_enable_write_console_messages_to_stdout(s, FALSE);
	webkit_settings_set_enable_page_cache(s, FALSE);
	webkit_settings_set_enable_html5_local_storage(s, FALSE);
	webkit_settings_set_enable_html5_database(s, FALSE);
	webkit_settings_set_javascript_can_open_windows_automatically(s, FALSE);
	webkit_settings_set_enable_smooth_scrolling(s, TRUE);

	GdkRGBA clear = {0, 0, 0, 0};
	webkit_web_view_set_background_color(view, &clear);
	gtk_container_add(GTK_CONTAINER(win), GTK_WIDGET(view));

	g_signal_connect(view, "decide-policy", G_CALLBACK(on_policy), NULL);
	g_signal_connect(view, "context-menu", G_CALLBACK(on_context_menu), NULL);
	g_signal_connect(win, "delete-event", G_CALLBACK(on_delete), NULL);

	webkit_web_view_load_html(view, html, NULL);
	// Realized but not shown: the page loads now, so the first open is
	// as quick as every other.
	gtk_widget_realize(win);
	gtk_widget_show(GTK_WIDGET(view));
}

void window_run(void) {
	gtk_main();
}

struct show_args {
	double x, y;
	int has_monitor;
};

// pick_monitor finds the GDK monitor whose layout position matches the
// one Hyprland named, so the panel opens on the monitor that was clicked.
static GdkMonitor *pick_monitor(double x, double y) {
	GdkDisplay *d = gdk_display_get_default();
	int n = gdk_display_get_n_monitors(d);
	for (int i = 0; i < n; i++) {
		GdkMonitor *m = gdk_display_get_monitor(d, i);
		GdkRectangle g;
		gdk_monitor_get_geometry(m, &g);
		if ((int)(x + 0.5) == g.x && (int)(y + 0.5) == g.y) {
			return m;
		}
	}
	return NULL;
}

static gboolean show_idle(gpointer data) {
	struct show_args *a = data;
	if (a->has_monitor) {
		gtk_layer_set_monitor(GTK_WINDOW(win), pick_monitor(a->x, a->y));
	} else {
		gtk_layer_set_monitor(GTK_WINDOW(win), NULL);
	}
	gtk_widget_show(win);
	gtk_window_present(GTK_WINDOW(win));
	gtk_widget_grab_focus(GTK_WIDGET(view));
	g_free(a);
	return G_SOURCE_REMOVE;
}

// Safe from any thread.
void window_show(double monitor_x, double monitor_y, int has_monitor) {
	struct show_args *a = g_new(struct show_args, 1);
	a->x = monitor_x;
	a->y = monitor_y;
	a->has_monitor = has_monitor;
	g_idle_add(show_idle, a);
}

static gboolean hide_idle(gpointer data) {
	gtk_widget_hide(win);
	return G_SOURCE_REMOVE;
}

// Safe from any thread.
void window_hide(void) {
	g_idle_add(hide_idle, NULL);
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
