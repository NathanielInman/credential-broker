package internal

import "github.com/fatih/color"

// Log outputs a formatted log line: [ip]<name>: description
// If failure is true, adds [FAILURE] prefix to description in red.
func Log(ip, name, description string, failure bool) {
	cyan := color.New(color.FgCyan).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()

	prefix := "[" + cyan(ip) + "]<" + magenta(name) + ">: "

	if failure {
		color.New().Print(prefix + red("[FAILURE] ") + red(description) + "\n")
	} else {
		color.New().Print(prefix + green(description) + "\n")
	}
}
