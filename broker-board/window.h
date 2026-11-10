#ifndef BROKER_BOARD_WINDOW_H
#define BROKER_BOARD_WINDOW_H

void window_create(const char *html);
void window_run(void);
void window_show(const char *monitor);
void window_hide(void);
void window_eval(const char *js);
void window_quit(void);

#endif
