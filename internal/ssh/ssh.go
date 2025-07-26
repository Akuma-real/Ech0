package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/lin-snow/ech0/internal/config"
	"github.com/lin-snow/ech0/internal/tui"
)

var SSHServer *ssh.Server

func SSHStart() {
	host := config.Config.SSH.Host
	port := config.Config.SSH.Port
	key := config.Config.SSH.Key
	
	var err error

	SSHServer, err = wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(key),
		wish.WithMiddleware(
			BubbleteaMiddleware(teaHandler),
			ActivetermMiddleware(), // Bubble Tea apps usually require a PTY.
		),
	)
	if err != nil {
		// log.Error("Could not start server", "error", err)
	}

	// done := make(chan os.Signal, 1)
	// signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	// log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		fmt.Println("🚀 Ech0 SSH已启动，监听端口", port)
		if err = SSHServer.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			// log.Error("Could not start server", "error", err)
			// done <- nil
		}
	}()

	// <-done
	// // log.Info("Stopping SSH server")
	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer func() { cancel() }()
	// if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
	// 	// log.Error("Could not stop server", "error", err)
	// }
}

func SSHStop() error {
	if SSHServer == nil {
		return nil
	}

	// When it arrives, we create a context with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()

	// When we start the shutdown, the server will no longer accept new
	// connections, but will wait as much as the given context allows for the
	// active connections to finish.
	// After the timeout, it shuts down anyway.
	if err := SSHServer.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		// 强制关闭服务器
		SSHServer.Close()

		return err
	}

	SSHServer = nil // Clear the server instance
	return nil
}

// Middleware will exit 1 connections trying with no active terminals.
func ActivetermMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			_, _, active := sess.Pty()
			if active {
				next(sess)
				return
			}
			wish.Println(sess, "Requires an active PTY")
			_ = sess.Exit(1)
		}
	}
}

// You can wire any Bubble Tea model up to the middleware with a function that
// handles the incoming ssh.Session. Here we just grab the terminal info and
// pass it to the new model. You can also return tea.ProgramOptions (such as
// tea.WithAltScreen) on a session by session basis.
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// This should never fail, as we are using the activeterm middleware.
	pty, _, _ := s.Pty()

	// When running a Bubble Tea app over SSH, you shouldn't use the default
	// lipgloss.NewStyle function.
	// That function will use the color profile from the os.Stdin, which is the
	// server, not the client.
	// We provide a MakeRenderer function in the bubbletea middleware package,
	// so you can easily get the correct renderer for the current session, and
	// use it to create the styles.
	// The recommended way to use these styles is to then pass them down to
	// your Bubble Tea model.
	renderer := MakeRenderer(s)
	txtStyle := renderer.NewStyle().Foreground(lipgloss.Color("10"))
	quitStyle := renderer.NewStyle().Foreground(lipgloss.Color("8"))

	bg := "light"
	if renderer.HasDarkBackground() {
		bg = "dark"
	}

	m := model{
		term:      pty.Term,
		profile:   renderer.ColorProfile().Name(),
		width:     pty.Window.Width,
		height:    pty.Window.Height,
		bg:        bg,
		txtStyle:  txtStyle,
		quitStyle: quitStyle,
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	term      string
	profile   string
	width     int
	height    int
	bg        string
	txtStyle  lipgloss.Style
	quitStyle lipgloss.Style
	logo      string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := tui.GetLogoBanner()
	return m.txtStyle.Render(s) + "\n\n" + m.quitStyle.Render("按 'q' 退出\n")
}


