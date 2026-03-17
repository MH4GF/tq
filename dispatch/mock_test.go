package dispatch

import "context"

type mockCall struct {
	ctx  context.Context
	name string
	args []string
	dir  string
	env  []string
}

type mockRunner struct {
	calls  []mockCall
	output []byte
	err    error
	// failAt indicates which call index should return the error (0-based). -1 means no failure.
	failAt int
}

func (m *mockRunner) Run(ctx context.Context, name string, args []string, dir string, env []string) ([]byte, error) {
	idx := len(m.calls)
	m.calls = append(m.calls, mockCall{ctx: ctx, name: name, args: args, dir: dir, env: env})
	if m.failAt >= 0 && idx == m.failAt {
		return m.output, m.err
	}
	return m.output, nil
}
