package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// Text prompts for text input with a label, returns the entered string.
func Text(label string) (string, error) {
	fmt.Print(label + ": ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input received")
	}
	return scanner.Text(), nil
}

// Password prompts for masked password input (no echo), returns the entered string.
func Password(label string) (string, error) {
	fmt.Print(label + ": ")
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after masked input
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Confirm prompts for yes/no confirmation, returns true for yes.
func Confirm(label string) (bool, error) {
	fmt.Printf("%s (y/n): ", label)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, err
		}
		return false, fmt.Errorf("no input received")
	}
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return input == "y" || input == "yes", nil
}

// Select prompts user to select from a list of options, returns the selected option string.
func Select(label string, options []string) (string, error) {
	fmt.Println(label)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}
	fmt.Print("Enter number: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input received")
	}

	num, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return "", fmt.Errorf("invalid selection: %w", err)
	}
	if num < 1 || num > len(options) {
		return "", fmt.Errorf("selection out of range: %d", num)
	}

	return options[num-1], nil
}

// Multiline prompts for multiline text input (empty line to finish), returns the full text.
func Multiline(label string) (string, error) {
	fmt.Println(label + " (empty line to finish):")

	scanner := bufio.NewScanner(os.Stdin)
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return strings.Join(lines, "\n"), nil
}
