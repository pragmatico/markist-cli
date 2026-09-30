package cli

import "errors"

// ErrNotLoggedIn is returned by whoami (and any future command that
// requires a session) when the config has no saved access token at all --
// distinct from api.ErrSessionExpired, which covers a token that existed
// but was rejected by the server. Both map to exit code 3.
var ErrNotLoggedIn = errors.New("not logged in. Run `markist login`")
