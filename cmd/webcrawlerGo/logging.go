package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
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

	textColorMap = map[string]*lipgloss.Style{
		"Error":   &redStyle,
		"Invalid": &redStyle,
		"FATAL":   &redStyle,
		"Added":   &greenStyle,
		"Saved":   &greenStyle,
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
	teaProgram   *tea.Program
	mu           sync.Mutex
	crawlerCount int
}

// Log sends msg to [tea.Program]
func (cl *crawLogger) Log(msg string) {
	// cl.mu.Lock()
	// defer cl.mu.Unlock()
	msgSlice := strings.Split(msg, ":")
	msgSlice[0] = cyanStyle.Render(msgSlice[0])

	msg = strings.Join(msgSlice, ":")

	cl.teaProgram.Send(colorAroundTexts(msg, textColorMap))
}

// Quit will quit [tea.Program] when last crawler quits
func (cl *crawLogger) Quit() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.crawlerCount--
	// quit when last crawler exits
	if cl.crawlerCount == 0 {
		ctrlC := tea.KeyMsg{
			Type: tea.KeyCtrlC,
		}
		cl.teaProgram.Send(ctrlC)
	}
}

// initialiseLoggers returns a log file handle f and a MultiWriter logger (os.Stdout & f)
func initialiseLoggers(verbose bool) (*os.File, *loggers) {
	logFileName := fmt.Sprintf(
		"./%s/logfile-%s.log",
		logFolderName,
		time.Now().Format("02-01-2006-15-04-05"),
	)
	f, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	logFlags := log.LstdFlags
	if verbose {
		logFlags = log.LstdFlags | log.Lshortfile
	}
	loggers := &loggers{
		multiLogger: log.New(io.MultiWriter(os.Stdout, f), "", logFlags),
		fileLogger:  log.New(f, "", logFlags),
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
		s += cyanStyle.Render("Aaand... we're done!")
	} else {
		s += m.spinner.View() + redStyle.Render(" Crawlers be crawling...")
	}

	s += "\n\n"

	for _, msg := range m.messages {
		s += msg + "\n"
	}

	if !m.quitting {
		s += helpStyle.Render("Press Ctrl + c or q to quit")
	}

	if m.quitting {
		// send SIGINT to quitChan to cancel the main context in listenForSignals func
		m.quitChan <- syscall.SIGINT
		s += "\n" + redStyle.Render("Waiting for crawlers to quit... ") + "\n\n"
	}

	return appStyle.Render(s)
}

// printAndLog will print msg to [os.Stdout] using printFunc
// and write to logFile. printFunc will print on new line and
// new line will be added to msg while writing to logFile
func printAndLog(printFunc func(string), logFile *os.File, msg string) {
	printFunc(msg)
	logFile.Write([]byte(msg + "\n"))
}
