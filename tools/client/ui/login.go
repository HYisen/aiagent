package ui

import (
	"bufio"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"log/slog"
	"os"
	"syscall"
	"time"
)

func loginTerminal() (username string, password string, err error) {
	oldState, err := term.MakeRaw(syscall.Stdin)
	defer func() {
		err = errors.Join(err, term.Restore(syscall.Stdin, oldState))
	}()

	screen := struct {
		io.Reader
		io.Writer
	}{os.Stdin, os.Stdout}
	terminal := term.NewTerminal(screen, "")

	_, _ = fmt.Fprint(terminal, string(terminal.Escape.Magenta))
	_, _ = fmt.Fprintln(terminal, "Server responds 403. Try login...")
	_, _ = fmt.Fprint(terminal, string(terminal.Escape.Reset))
	_, _ = fmt.Fprint(terminal, "Login: ")
	username, err = terminal.ReadLine()
	if err != nil {
		return "", "", err
	}
	password, err = terminal.ReadPassword("Password: ")
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

func loginFallback() (username string, password string, err error) {
	time.Sleep(time.Second) // wait to expect log output done
	fmt.Println("Server responds 403. Try login...")
	fmt.Print("Login: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username = scanner.Text()
	fmt.Print("Password: ")
	scanner.Scan()
	password = scanner.Text()
	return username, password, scanner.Err()
}

func isTerminal() bool {
	return term.IsTerminal(syscall.Stdin) && term.IsTerminal(syscall.Stdout)
}

func Login() (username string, password string, err error) {
	if !isTerminal() {
		slog.Warn("Not terminal, switch to fallback password echo mode.")
		return loginFallback()
	} else {
		return loginTerminal()
	}
}
