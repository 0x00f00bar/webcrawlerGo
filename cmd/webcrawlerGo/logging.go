package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle      = helpStyle.UnsetMargins()
	durationStyle = dotStyle
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
)

// crawLogger sends the events received to
// [tea.Program]
type crawLogger struct {
	teaProgram   *tea.Program
	mu           sync.Mutex
	crawlerCount int
}

// Write implements [io.Writer] for crawLogger type
func (cl *crawLogger) Write(p []byte) (n int, err error) {
	// cl.mu.Lock()
	// defer cl.mu.Unlock()
	cl.teaProgram.Send(string(p))
	return len(p), nil
}

func (cl *crawLogger) Quit() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	// quit when last crawler exits
	if cl.crawlerCount > 1 {
		cl.crawlerCount--
	} else {
		cl.teaProgram.Quit()
	}
}

// initialiseLogger returns a log file handle f and a MultiWriter logger (os.Stdout & f)
func initialiseLogger() (f *os.File, logger *log.Logger) {
	logFileName := fmt.Sprintf(
		"./%s/logfile-%s.log",
		logFolderName,
		time.Now().Format("02-01-2006-15-04-05"),
	)
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	// not using with bubbletea Program
	// return f, log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags|log.Lshortfile)
	return f, log.New(f, "", log.LstdFlags|log.Lshortfile)
}

type model struct {
	spinner  spinner.Model
	messages []string
	quitting bool
}

func newModel(numMsgs int) model {
	s := spinner.New()
	s.Style = spinnerStyle
	return model{
		spinner:  s,
		messages: make([]string, numMsgs),
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case string:
		m.messages = append(m.messages[1:], msg)
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m model) View() string {
	var s string

	if m.quitting {
		s += "Aaand... we're done!"
	} else {
		s += m.spinner.View() + " Crawlers be crawling..."
	}

	s += Reset + "\n\n"

	for _, msg := range m.messages {
		s += msg + "\n"
	}

	if !m.quitting {
		s += helpStyle.Render("Press Ctrl + c or q to quit")
	}

	if m.quitting {
		s += "\n" + Red + "Waiting for crawlers... " + Reset + "\n\n"
	}

	return appStyle.Render(s)
}
