package cli

// Process exit codes (plan Task 9). Not every command wires all of these
// yet -- ExitNotLoggedIn and ExitUpgradeRequired are mapped from typed
// api/auth errors starting in Task 10.
const (
	ExitSuccess         = 0
	ExitError           = 1
	ExitUsage           = 2
	ExitNotLoggedIn     = 3
	ExitUpgradeRequired = 4
)
