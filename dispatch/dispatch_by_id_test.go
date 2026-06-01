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
			name:   "interactive shows agent view short",
			result: ExecuteResult{Mode: ModeInteractive, Output: "abcd1234"},
			want:   "action #42 dispatched to claude agent view (short: abcd1234)",
		},
		{
			name:   "noninteractive shows agent view short",
			result: ExecuteResult{Mode: ModeNonInteractive, Output: "abcd1234"},
			want:   "action #42 dispatched to claude agent view (short: abcd1234)",
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
