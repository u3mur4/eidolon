package main

import "github.com/fatih/color"

// Colors holds all color definitions for output formatting
type Colors struct {
	Header      *color.Color
	Cmd         *color.Color
	Stdin       *color.Color
	Stdout      *color.Color
	Stderr      *color.Color
	Hex         *color.Color
	ExitCode    *color.Color
	Flag        *color.Color
	FlagVal     *color.Color
	Arg         *color.Color
	SpecialChar *color.Color
	Highlight   *color.Color
}

// NewColors creates a new Colors instance with all color definitions
func NewColors() *Colors {
	return &Colors{
		Header:      color.New(color.FgCyan, color.Bold),
		Cmd:         color.New(color.FgYellow),
		Stdin:       color.New(color.FgGreen),
		Stdout:      color.New(color.FgWhite),
		Stderr:      color.New(color.FgRed),
		Hex:         color.New(color.Faint),
		ExitCode:    color.New(color.BgRed, color.FgYellow, color.Bold),
		Flag:        color.New(color.FgHiCyan),
		FlagVal:     color.New(color.FgCyan),
		Arg:         color.New(color.FgHiGreen),
		SpecialChar: color.New(color.FgHiMagenta, color.Bold),
		Highlight:   color.New(color.BgYellow, color.FgBlack),
	}
}
