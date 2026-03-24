package main

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/fatih/color"
	"github.com/u3mur4/eidolon/pkg/common/types"
)

// Segment represents a piece of text with styling
type Segment struct {
	Text  string
	Style Style
}

// Style controls the visual representation of a segment
type Style struct {
	BaseColor   *color.Color // stdin/stderr/stdout base color
	Highlight   bool         // search term highlighted
	SpecialChar bool         // null bytes, special chars
}

// LogFormatter handles all output formatting for log messages
type LogFormatter struct {
	Colors     *Colors
	SearchText string
	EnvMode    string
	ServerEnv  []string
}

// NewLogFormatter creates a new formatter with the given colors and search text
func NewLogFormatter(colors *Colors, searchText, envMode string, serverEnv []string) *LogFormatter {
	return &LogFormatter{
		Colors:     colors,
		SearchText: searchText,
		EnvMode:    envMode,
		ServerEnv:  serverEnv,
	}
}

// Format formats and prints a complete log message using the segment pipeline
func (f *LogFormatter) Format(msg *types.LogMessage) string {
	var sb strings.Builder

	// Header
	hColor := f.Colors.Header
	if msg.ExitCode != 0 {
		hColor = f.Colors.ExitCode
	}

	cmdName := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdName = fmt.Sprintf("%s -> %s", msg.Alias, msg.Path)
	}

	statusText := "running"
	if msg.Status == "completed" {
		statusText = fmt.Sprintf("exit(%d)", msg.ExitCode)
	}

	startTime := msg.StartTime.Format("15:04:05.000")
	var elapsed string
	if msg.ExitTime.IsZero() {
		elapsed = f.formatDuration(time.Since(msg.StartTime))
	} else {
		elapsed = f.formatDuration(msg.ExitTime.Sub(msg.StartTime))
	}

	headerText := fmt.Sprintf("PID: %d |PPID: %d |CMD: %s |STATUS: %s |START: %s |ELAPSED: %s\n",
		msg.PID, msg.PPID, cmdName, statusText, startTime, elapsed)

	headerSegments := f.tokenize(headerText, hColor)
	headerSegments = f.applyStyles(headerSegments)
	f.render(&sb, headerSegments)

	// Environment variables
	envSegments := f.formatEnvSegments(msg.Env)
	if len(envSegments) > 0 {
		envSegments = f.applyStyles(envSegments)
		f.render(&sb, envSegments)
	}

	// Arguments
	cmdToDisplay := msg.Alias
	if msg.Path != "" && msg.Alias != filepath.Base(msg.Path) {
		cmdToDisplay = msg.Path
	}

	var cmdSegments[]Segment
	cmdSegments = append(cmdSegments, f.tokenize(cmdToDisplay, f.Colors.Cmd)...)
	if len(msg.Args) > 0 {
		cmdSegments = append(cmdSegments, Segment{Text: " ", Style: Style{BaseColor: f.Colors.Cmd}})
		cmdSegments = append(cmdSegments, f.formatArgsForDisplaySegments(msg.Args)...)
	}
	cmdSegments = append(cmdSegments, Segment{Text: "\n", Style: Style{BaseColor: f.Colors.Cmd}})

	cmdSegments = f.applyStyles(cmdSegments)
	f.render(&sb, cmdSegments)

	// Stdin, Stdout, Stderr
	sb.WriteString(f.formatData(f.Colors.Stdin, "STDIN", msg.StdinData))
	sb.WriteString(f.formatData(f.Colors.Stdout, "STDOUT", msg.StdoutData))
	sb.WriteString(f.formatData(f.Colors.Stderr, "STDERR", msg.StderrData))

	// Separator
	separatorSegments := f.tokenize(strings.Repeat("-", 80)+"\n", f.Colors.Header)
	f.render(&sb, separatorSegments)

	return sb.String()
}

// tokenize breaks text into safe-to-print string chunks and identifies non-printable special chars
func (f *LogFormatter) tokenize(text string, baseColor *color.Color) []Segment {
	var segments[]Segment
	var currentText strings.Builder

	for i := 0; i < len(text); i++ {
		b := text[i]
		if b >= 32 && b < 127 || unicode.IsSpace(rune(b)) {
			currentText.WriteByte(b)
		} else {
			if currentText.Len() > 0 {
				segments = append(segments, Segment{
					Text:  currentText.String(),
					Style: Style{BaseColor: baseColor},
				})
				currentText.Reset()
			}
			segments = append(segments, Segment{
				Text:  fmt.Sprintf("\\x%02x", b),
				Style: Style{BaseColor: baseColor, SpecialChar: true},
			})
		}
	}
	if currentText.Len() > 0 {
		segments = append(segments, Segment{
			Text:  currentText.String(),
			Style: Style{BaseColor: baseColor},
		})
	}
	return segments
}

// applyStyles maps search terms onto existing segments and breaks them up if necessary
func (f *LogFormatter) applyStyles(segments []Segment)[]Segment {
	if f.SearchText == "" || len(segments) == 0 {
		return segments
	}

	var combined strings.Builder
	for _, seg := range segments {
		combined.WriteString(seg.Text)
	}
	fullText := combined.String()

	var matches [][2]int
	start := 0
	for {
		idx := strings.Index(fullText[start:], f.SearchText)
		if idx == -1 {
			break
		}
		matchStart := start + idx
		matchEnd := matchStart + len(f.SearchText)
		matches = append(matches, [2]int{matchStart, matchEnd})
		start = matchEnd // Advance past this match
	}

	if len(matches) == 0 {
		return segments
	}

	var result[]Segment
	charIndex := 0

	for _, seg := range segments {
		segLen := len(seg.Text)
		segStart := charIndex
		segEnd := charIndex + segLen
		charIndex = segEnd

		segOffset := 0
		for segOffset < segLen {
			absOffset := segStart + segOffset

			inMatch := false
			matchEndAbs := -1
			nextMatchStart := -1

			// Find if offset falls within a match or where the next match starts
			for _, m := range matches {
				if absOffset >= m[0] && absOffset < m[1] {
					inMatch = true
					matchEndAbs = m[1]
					break
				} else if m[0] > absOffset {
					if nextMatchStart == -1 || m[0] < nextMatchStart {
						nextMatchStart = m[0]
					}
				}
			}

			if inMatch {
				end := matchEndAbs
				if segEnd < end {
					end = segEnd
				}
				length := end - absOffset

				subSeg := seg
				subSeg.Text = seg.Text[segOffset : segOffset+length]
				subSeg.Style.Highlight = true
				result = append(result, subSeg)

				segOffset += length
			} else {
				end := segEnd
				if nextMatchStart != -1 && nextMatchStart < end {
					end = nextMatchStart
				}
				length := end - absOffset

				subSeg := seg
				subSeg.Text = seg.Text[segOffset : segOffset+length]
				result = append(result, subSeg)

				segOffset += length
			}
		}
	}

	return result
}

// render iterates over styled segments and applies ANSI codes in a single pass based on priority
func (f *LogFormatter) render(sb *strings.Builder, segments[]Segment) {
	for _, seg := range segments {
		if seg.Text == "" {
			continue
		}

		var c *color.Color

		// Priority Rules (highest wins)
		if seg.Style.Highlight && f.Colors.Highlight != nil {
			c = f.Colors.Highlight
		} else if seg.Style.SpecialChar && f.Colors.SpecialChar != nil {
			c = f.Colors.SpecialChar
		} else if seg.Style.BaseColor != nil {
			c = seg.Style.BaseColor
		}

		if c != nil {
			sb.WriteString(c.Sprint(seg.Text))
		} else {
			sb.WriteString(seg.Text)
		}
	}
}

// formatData handles STDIN/STDOUT/STDERR payloads, properly managing printability and hex dumps
func (f *LogFormatter) formatData(c *color.Color, title string, data[]byte) string {
	if len(data) == 0 {
		return ""
	}

	var sb strings.Builder

	titleText := fmt.Sprintf("%s (%d bytes)\n", title, len(data))
	titleSegments := f.tokenize(titleText, c)
	f.render(&sb, titleSegments)

	if f.isPrintable(data) {
		dataStr := string(data)
		if !strings.HasSuffix(dataStr, "\n") {
			dataStr += "\n"
		}
		segments := f.tokenize(dataStr, c)
		segments = f.applyStyles(segments)
		f.render(&sb, segments)
	} else {
		dump := hex.Dump(data)
		segments := f.tokenize(dump, c)
		segments = f.applyStyles(segments)
		f.render(&sb, segments)
	}

	return sb.String()
}

// formatArgsForDisplaySegments parses args into structured segments containing correct styles
func (f *LogFormatter) formatArgsForDisplaySegments(args []string) []Segment {
	var segments[]Segment
	prevWasFlag := false

	for i, arg := range args {
		if i > 0 {
			segments = append(segments, Segment{Text: " ", Style: Style{BaseColor: f.Colors.Cmd}})
		}

		if strings.HasPrefix(arg, "-") {
			if idx := strings.Index(arg, "="); idx != -1 {
				// flag=value
				flagPart := arg[:idx]
				valPart := arg[idx+1:]

				segments = append(segments, f.tokenize(flagPart, f.Colors.Flag)...)
				segments = append(segments, Segment{Text: "=", Style: Style{BaseColor: f.Colors.Stdout}})
				segments = append(segments, f.tokenize(valPart, f.Colors.FlagVal)...)

				prevWasFlag = false
			} else {
				// simple flag
				segments = append(segments, f.tokenize(arg, f.Colors.Flag)...)
				prevWasFlag = true
			}
		} else {
			// not a flag
			var base *color.Color
			if prevWasFlag {
				base = f.Colors.Arg
			} else {
				base = f.Colors.Cmd
			}
			segments = append(segments, f.tokenize(arg, base)...)
			prevWasFlag = false
		}
	}

	return segments
}

// formatEnvSegments applies specific diff logic/formatting to environment variables
func (f *LogFormatter) formatEnvSegments(env []string)[]Segment {
	if f.EnvMode == "none" || len(env) == 0 {
		return nil
	}

	envToPrint := env

	if f.EnvMode == "diff" {
		serverEnvSet := make(map[string]bool)
		for _, e := range f.ServerEnv {
			if idx := strings.Index(e, "="); idx != -1 {
				serverEnvSet[e[:idx]] = true
			}
		}

		var filtered []string
		for _, e := range env {
			if idx := strings.Index(e, "="); idx != -1 {
				key := e[:idx]
				if !serverEnvSet[key] {
					filtered = append(filtered, e)
				}
			}
		}
		envToPrint = filtered
	}

	if len(envToPrint) == 0 {
		return nil
	}

	envText := strings.Join(envToPrint, " ") + "\n"
	return f.tokenize(envText, f.Colors.Env)
}

func (f *LogFormatter) isPrintable(data []byte) bool {
	if len(data) == 0 {
		return true
	}

	printable := 0
	for _, b := range data {
		if (b >= 32 && b < 127) || unicode.IsSpace(rune(b)) {
			printable++
		}
	}
	return float64(printable)/float64(len(data)) > 0.9
}

// formatDuration converts a duration into a human-readable string with appropriate precision
func (f *LogFormatter) formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		// < 1ms: show microseconds with 1 decimal
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000.0)
	} else if d < time.Second {
		// 1ms - 1s: show milliseconds with 1 decimal
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
	} else if d < 60*time.Second {
		// 1s - 60s: show seconds with 1 decimal
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else {
		// >= 60s: show minutes and seconds
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if mins > 0 && secs > 0 {
			return fmt.Sprintf("%dm%ds", mins, secs)
		} else if mins > 0 {
			return fmt.Sprintf("%dm", mins)
		} else {
			return fmt.Sprintf("%ds", secs)
		}
	}
}
