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

// sequenceRunner returns successive outputs across calls; the last entry repeats.
type sequenceRunner struct {
	outputs [][]byte
	idx     int
}

func (s *sequenceRunner) Run(_ context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
	out := s.outputs[s.idx]
	if s.idx < len(s.outputs)-1 {
		s.idx++
	}
	return out, nil
}
