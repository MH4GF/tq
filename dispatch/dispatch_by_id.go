package dispatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DispatchMessage returns the human-readable status line for a completed
// dispatch of action id. Shared by `tq action dispatch` and the `tq ui`
// dispatch keybinding so both report identical wording.
func (r *ExecuteResult) DispatchMessage(id int64) string {
	switch r.Mode {
	case ModeRemote:
		url := strings.TrimPrefix(r.Output, RemoteSessionPrefix)
		return fmt.Sprintf("action #%d dispatched remotely (view: %s)", id, url)
	case ModeInteractive:
		return fmt.Sprintf("action #%d dispatched interactively (%s)", id, r.Output)
	case ModeBg:
		return fmt.Sprintf("action #%d dispatched to claude agent view (short: %s)", id, r.Output)
	default:
		return fmt.Sprintf("action #%d done", id)
	}
}

// DispatchByID claims a pending action and executes it, returning a
// human-readable status line. An ActionFailedError is reported as a message
// (not an error), matching the manual-override semantics of
// `tq action dispatch`: the action ran but its work failed.
func DispatchByID(ctx context.Context, cfg DispatchConfig, id int64) (string, error) {
	action, err := cfg.DB.ClaimPending(ctx, id)
	if err != nil {
		return "", err
	}
	result, err := ExecuteAction(ctx, ExecuteParams{DispatchConfig: cfg}, action)
	var af *ActionFailedError
	if errors.As(err, &af) {
		return fmt.Sprintf("action #%d failed (%v)", af.ActionID, af.Err), nil
	}
	if err != nil {
		return "", err
	}
	return result.DispatchMessage(action.ID), nil
}
