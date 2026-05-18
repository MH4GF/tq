package dispatch

import "testing"

func TestExecuteResult_DispatchMessage(t *testing.T) {
	tests := []struct {
		name   string
		result ExecuteResult
		want   string
	}{
		{
			name:   "remote strips session prefix",
			result: ExecuteResult{Mode: ModeRemote, Output: RemoteSessionPrefix + "https://example.com/s/1"},
			want:   "action #42 dispatched remotely (view: https://example.com/s/1)",
		},
		{
			name:   "interactive",
			result: ExecuteResult{Mode: ModeInteractive, Output: "tq-action-42"},
			want:   "action #42 dispatched interactively (tq-action-42)",
		},
		{
			name:   "bg",
			result: ExecuteResult{Mode: ModeBg, Output: "abcd1234"},
			want:   "action #42 dispatched to claude agent view (short: abcd1234)",
		},
		{
			name:   "noninteractive falls through to done",
			result: ExecuteResult{Mode: ModeNonInteractive, Output: ""},
			want:   "action #42 done",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.DispatchMessage(42); got != tt.want {
				t.Errorf("DispatchMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
