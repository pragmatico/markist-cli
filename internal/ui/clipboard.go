package ui

import "github.com/atotto/clipboard"

// CopyToClipboard puts text on the system clipboard. Used by the picker's
// "c" key (plan Task 13).
func CopyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}
