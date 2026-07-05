// Package ui centralises terminal styling so every command speaks the
// same visual language.
package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	Accent  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	Good    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	Warn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Bad     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	Dim     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	Header  = lipgloss.NewStyle().Bold(true).Underline(true)
	KeyCell = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
)

func Successf(format string, a ...any) { fmt.Println(Good.Render("✓ ") + fmt.Sprintf(format, a...)) }
func Warnf(format string, a ...any)    { fmt.Println(Warn.Render("! ") + fmt.Sprintf(format, a...)) }
func Errorf(format string, a ...any) {
	fmt.Fprintln(os.Stderr, Bad.Render("✗ ")+fmt.Sprintf(format, a...))
}
func Infof(format string, a ...any) { fmt.Println(Dim.Render("· ") + fmt.Sprintf(format, a...)) }
func Titlef(format string, a ...any) {
	fmt.Println(Accent.Render(fmt.Sprintf(format, a...)))
}
