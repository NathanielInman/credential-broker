// The Signal Box panel: a transparent layer-shell surface on the overlay
// layer that covers one monitor (less what other surfaces reserve), holding
// a WebKit view of the embedded page. The page draws the panel where the
// bar icon is and animates it open and closed; this file only maps and
// unmaps the surface on the right monitor. The page talks to Go through the
// "board" script message handler; Go talks to the page by evaluating
// board.receive().
//
// GTK 4 and WebKitGTK 6, not GTK 3: GTK 3 drops parts of a transparent
// surface's content between frames on Wayland, so the panel flickered to
// whatever was behind it. GTK 4 redraws the whole surface every frame.

#include <gtk/gtk.h>
#include <gtk4-layer-shell.h>
#include <webkit/webkit.h>

#include "window.h"
#include "_cgo_export.h"

static GtkWidget *win;
static WebKitWebView *view;
static GMainLoop *loop;

static void on_message(WebKitUserContentManager *m, JSCValue *v, gpointer data) {
	if (!jsc_value_is_string(v)) {
		return;
	}
	char *s = jsc_value_to_string(v);
	goMessage(s);
	g_free(s);
}

static gboolean on_close(GtkWindow *w, gpointer data) {
	gtk_widget_set_visible(win, FALSE);
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

static gboolean on_context_menu(WebKitWebView *v, WebKitContextMenu *m, WebKitHitTestResult *h, gpointer data) {
	return TRUE;
}

void window_create(const char *html) {
	g_set_prgname("broker-board");
	gtk_init();
	loop = g_main_loop_new(NULL, FALSE);

	// Every window of this program is see-through; the page paints the
	// panel and nothing else.
	GtkCssProvider *css = gtk_css_provider_new();
	gtk_css_provider_load_from_string(css, "window, window.background { background: transparent; box-shadow: none; }");
	gtk_style_context_add_provider_for_display(gdk_display_get_default(), GTK_STYLE_PROVIDER(css), GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);

	win = gtk_window_new();
	gtk_window_set_title(GTK_WINDOW(win), "Signal Box");
	gtk_window_set_decorated(GTK_WINDOW(win), FALSE);
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

	WebKitUserContentManager *m = webkit_user_content_manager_new();
	g_signal_connect(m, "script-message-received::board", G_CALLBACK(on_message), NULL);
	webkit_user_content_manager_register_script_message_handler(m, "board", NULL);
	view = WEBKIT_WEB_VIEW(g_object_new(WEBKIT_TYPE_WEB_VIEW, "user-content-manager", m, NULL));

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
	gtk_window_set_child(GTK_WINDOW(win), GTK_WIDGET(view));

	g_signal_connect(view, "decide-policy", G_CALLBACK(on_policy), NULL);
	g_signal_connect(view, "context-menu", G_CALLBACK(on_context_menu), NULL);
	g_signal_connect(win, "close-request", G_CALLBACK(on_close), NULL);

	webkit_web_view_load_html(view, html, NULL);
	// Realized but not shown: the page loads now, so the first open is
	// as quick as every other.
	gtk_widget_realize(win);
}

void window_run(void) {
	g_main_loop_run(loop);
}

// pick_monitor finds the monitor Hyprland named by its connector (DP-2),
// so the panel opens on the monitor whose bar was clicked.
static GdkMonitor *pick_monitor(const char *name) {
	if (name == NULL || name[0] == '\0') {
		return NULL;
	}
	GListModel *list = gdk_display_get_monitors(gdk_display_get_default());
	guint n = g_list_model_get_n_items(list);
	for (guint i = 0; i < n; i++) {
		GdkMonitor *m = g_list_model_get_item(list, i);
		gboolean match = g_strcmp0(gdk_monitor_get_connector(m), name) == 0;
		g_object_unref(m);
		if (match) {
			return m; // the display's list still holds it
		}
	}
	return NULL;
}

static gboolean show_idle(gpointer data) {
	char *name = data;
	gtk_layer_set_monitor(GTK_WINDOW(win), pick_monitor(name));
	gtk_widget_set_visible(win, TRUE);
	gtk_window_present(GTK_WINDOW(win));
	gtk_widget_grab_focus(GTK_WIDGET(view));
	g_free(name);
	return G_SOURCE_REMOVE;
}

// Safe from any thread.
void window_show(const char *monitor) {
	g_idle_add(show_idle, g_strdup(monitor));
}

static gboolean hide_idle(gpointer data) {
	gtk_widget_set_visible(win, FALSE);
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
	g_main_loop_quit(loop);
	return G_SOURCE_REMOVE;
}

// Safe from any thread.
void window_quit(void) {
	g_idle_add(quit_idle, NULL);
}
