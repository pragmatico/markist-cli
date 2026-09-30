package ui

import "github.com/charmbracelet/huh"

// Confirm shows a Huh yes/no prompt (e.g. "Already logged in as @user. Log
// in again?" in plan Task 11) and returns the user's choice.
func Confirm(title string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(title).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		Run()
	if err != nil {
		return false, err
	}
	return confirmed, nil
}
