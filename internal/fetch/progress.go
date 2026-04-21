// Copyright (c) 2026 Neomantra Corp

package fetch

import (
	"io"
	"os"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// progressReporter reports download progress to stderr when stderr is a TTY,
// and is a no-op otherwise.
type progressReporter struct {
	prog  *tea.Program
	total int64
}

func newProgressReporter(totalBytes int64) *progressReporter {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return &progressReporter{}
	}
	m := &progressModel{bar: progress.New(progress.WithDefaultBlend()), total: totalBytes}
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	go func() { _, _ = p.Run() }()
	return &progressReporter{prog: p, total: totalBytes}
}

func (r *progressReporter) wrap(body io.Reader) io.Reader {
	if r.prog == nil {
		return body
	}
	return &countingReader{src: body, reporter: r}
}

func (r *progressReporter) update(n int64) {
	if r.prog == nil {
		return
	}
	r.prog.Send(progressMsg{bytesRead: n})
}

func (r *progressReporter) finish() {
	if r.prog == nil {
		return
	}
	r.prog.Quit()
	r.prog.Wait()
}

type countingReader struct {
	src      io.Reader
	reporter *progressReporter
	read     int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.src.Read(p)
	c.read += int64(n)
	c.reporter.update(c.read)
	return n, err
}

type progressMsg struct{ bytesRead int64 }

type progressModel struct {
	bar   progress.Model
	total int64
}

func (m *progressModel) Init() tea.Cmd { return nil }

func (m *progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		var pct float64
		if m.total > 0 {
			pct = float64(msg.bytesRead) / float64(m.total)
			if pct > 1 {
				pct = 1
			}
		}
		return m, m.bar.SetPercent(pct)
	case progress.FrameMsg:
		updatedBar, cmd := m.bar.Update(msg)
		m.bar = updatedBar
		return m, cmd
	case tea.QuitMsg:
		return m, nil
	}
	return m, nil
}

func (m *progressModel) View() tea.View {
	return tea.NewView(m.bar.View() + "\n")
}
