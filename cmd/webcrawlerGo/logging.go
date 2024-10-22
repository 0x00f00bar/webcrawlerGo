package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	dotStyle     = helpStyle.UnsetMargins()
	appStyle     = lipgloss.NewStyle().Margin(1, 2, 0, 2)

	textColorMap = map[string]string{
		"Error":   Red,
		"Invalid": Red,
		"FATAL":   Red,
		"Added":   Green,
		"Saved":   Green,
	}
)

// loggers stores multiple loggers
type loggers struct {
	fileLogger  *log.Logger
	multiLogger *log.Logger
}

// crawLogger sends the events received to
// [tea.Program]
type crawLogger struct {
	teaProgram *tea.Program
}

// Write implements [io.Writer] for crawLogger type
func (cl *crawLogger) Write(p []byte) (n int, err error) {
	// cl.mu.Lock()
	// defer cl.mu.Unlock()
	msgSlice := strings.Split(string(p), ":")
	msgSlice[0] = Cyan + msgSlice[0] + Reset

	msg := strings.Join(msgSlice, ":")

	cl.teaProgram.Send(colorAroundTexts(msg, textColorMap))
	return len(p), nil
}

// initialiseLoggers returns a log file handle f and a MultiWriter logger (os.Stdout & f)
func initialiseLoggers() (*os.File, *loggers) {
	logFileName := fmt.Sprintf(
		"./%s/logfile-%s.log",
		logFolderName,
		time.Now().Format("02-01-2006-15-04-05"),
	)
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	loggers := &loggers{
		multiLogger: log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags|log.Lshortfile),
		fileLogger:  log.New(f, "", log.LstdFlags|log.Lshortfile),
	}
	return f, loggers
}

// teaProgModel for [tea.Program]
type teaProgModel struct {
	spinner  spinner.Model
	messages []string
	quitting bool
	quitChan chan os.Signal
}

// newteaProgModel returns new teaProgModel
func newteaProgModel(numMsgs int, sigChan chan os.Signal) teaProgModel {
	s := spinner.New()
	s.Style = spinnerStyle
	messages := make([]string, numMsgs)
	dots := dotStyle.Render(strings.Repeat(".", 45))
	for i := range messages {
		messages[i] = dots
	}
	return teaProgModel{
		spinner:  s,
		messages: messages,
		quitChan: sigChan,
	}
}

func (m teaProgModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m teaProgModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m teaProgModel) View() string {
	var s string

	if m.quitting {
		s += Cyan + "Aaand... we're done!" + Reset
	} else {
		s += m.spinner.View() + Red + " Crawlers be crawling..." + Reset
	}

	s += Reset + "\n\n"

	for _, msg := range m.messages {
		s += msg + "\n"
	}

	if !m.quitting {
		s += helpStyle.Render("Press Ctrl + c or q to quit")
	}

	if m.quitting {
		m.quitChan <- syscall.SIGINT
		s += "\n" + Red + "Waiting for crawlers to quit... " + Reset + "\n\n"
	}

	return appStyle.Render(s)
}
