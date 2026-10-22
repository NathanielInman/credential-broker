#ifndef BROKER_DIALOG_WINDOW_H
#define BROKER_DIALOG_WINDOW_H

void window_create(const char *html, int width, int height);
void window_run(void);
void window_resize(int height);
void window_eval(const char *js);
void window_quit(void);

#endif
