package runtime

import "errors"

// ErrLoginCompleted is returned when the user finished logging in and must re-run
// the same command. Matches the legacy root PersistentPreRunE login-completed path.
var ErrLoginCompleted = errors.New("login completed successfully; please re-run your command")
