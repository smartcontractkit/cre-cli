//go:build unix

package prompt

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
)

// TODO - Move to a single cross-platform implementation using Bubble Tea or any other library that works on both Unix and Windows.

func SimplePrompt(reader io.Reader, promptText string, handler func(input string) error) error {
	prompt := promptui.Prompt{
		Label: promptText,
		Stdin: io.NopCloser(reader),
	}

	result, err := prompt.Run()
	if err != nil {
		return err
	}

	return handler(result)
}

func SelectPrompt(reader io.Reader, promptText string, choices []string, handler func(choice string) error) error {
	prompt := promptui.Select{
		Label: promptText,
		Items: choices,
		Stdin: io.NopCloser(reader),
	}

	_, result, err := prompt.Run()
	if err != nil {
		return err
	}

	return handler(result)
}

func YesNoPrompt(reader io.Reader, promptText string) (bool, error) {
	prompt := promptui.Select{
		Label: promptText,
		Items: []string{"Yes", "No"},
		Stdin: io.NopCloser(reader),
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false, err
	}

	return result == "Yes", nil
}

func SecretPrompt(reader io.Reader, promptText string, handler func(input string) error) error {
	prompt := promptui.Prompt{
		Label: promptText,
		Mask:  '*', // Mask input with '*'
		Stdin: io.NopCloser(reader),
	}

	// Run the prompt and get the result
	result, err := prompt.Run()
	if err != nil {
		return err
	}

	// Call the handler with the result
	return handler(result)
}

func UserPromptYesOrNoResponse() (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	input = strings.TrimSpace(input)
	input = strings.ToLower(input)

	switch input {
	case "y", "yes", "":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, errors.New("invalid input, please enter Y to continue or N to abort")
	}
}
