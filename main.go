package main

import (
	"bytes"
	"fmt"
	"image/color"
	"os/exec"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	a.Settings().SetTheme(&overlayTheme{})
	w := a.NewWindow("timew")
	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(300, 120))

	statusIcon := canvas.NewText("⏸", theme.Color(theme.ColorNameForeground))
	statusIcon.TextSize = 18
	statusIcon.TextStyle = fyne.TextStyle{Bold: true}

	statusLabel := widget.NewLabel("")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	dayTotal := widget.NewLabel("Day: —")
	dayTotal.TextStyle = fyne.TextStyle{Bold: true}

	elapsed := widget.NewLabel("")

	var toggleBtn *widget.Button
	var isTracking bool

	toggleBtn = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		if isTracking {
			runTimew("stop")
		} else {
			runTimew("start")
		}
		refresh(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
	})

	tagEntry := widget.NewEntry()
	tagEntry.SetPlaceHolder("tag")
	tagEntry.OnSubmitted = func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return
		}
		args := []string{"start"}
		args = append(args, strings.Fields(tag)...)
		runTimew(args...)
		tagEntry.SetText("")
		refresh(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
	}

	refresh(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(func() {
				refresh(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
			})
		}
	}()

	topRow := container.NewHBox(toggleBtn, statusIcon, statusLabel)
	bottomRow := container.NewHBox(dayTotal, layout.NewSpacer(), elapsed)

	content := container.NewVBox(
		topRow,
		bottomRow,
		widget.NewSeparator(),
		tagEntry,
	)

	w.SetContent(container.NewPadded(content))
	w.SetOnClosed(func() { a.Quit() })
	w.ShowAndRun()
}

func refresh(w fyne.Window, toggleBtn *widget.Button, icon *canvas.Text, status, dayTotal, elapsed *widget.Label, isTracking *bool) {
	tracking, currentTags, elapsedStr := getTimewStatus()
	total := getDayTotal()
	*isTracking = tracking

	if tracking {
		icon.Text = "⏺"
		icon.Color = parseHexColor("#4caf50")
		toggleBtn.SetIcon(theme.MediaStopIcon())
		if currentTags != "" {
			status.SetText(currentTags)
		} else {
			status.SetText("Tracking")
		}
		elapsed.SetText(elapsedStr)
		w.SetTitle(fmt.Sprintf("timew - %s (Active)", total))
	} else {
		icon.Text = "⏸"
		icon.Color = parseHexColor("#ff9800")
		toggleBtn.SetIcon(theme.MediaPlayIcon())
		status.SetText("Stopped")
		elapsed.SetText("")
		w.SetTitle(fmt.Sprintf("timew - %s (Stopped)", total))
	}
	icon.Refresh()

	dayTotal.SetText(fmt.Sprintf("Day: %s", total))
}

func getTimewStatus() (tracking bool, tags string, elapsed string) {
	out := runTimew()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 {
		return false, "", ""
	}

	first := lines[0]
	if strings.Contains(first, "no active") || strings.Contains(first, "There is no active") {
		return false, "", ""
	}

	if strings.HasPrefix(first, "Tracking") {
		tags = strings.TrimPrefix(first, "Tracking ")
		tags = strings.TrimSpace(tags)
		tags = strings.Trim(tags, "\"")
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Total") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				elapsed = parts[len(parts)-1]
			}
		}
	}

	return true, tags, elapsed
}

func getDayTotal() string {
	out := runTimew("summary", ":day")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 {
			last := fields[len(fields)-1]
			if isTimeDuration(last) {
				return last
			}
		}
	}
	return "0:00:00"
}

func isTimeDuration(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

func runTimew(args ...string) string {
	cmd := exec.Command("timew", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	return out.String()
}

func parseHexColor(hex string) color.Color {
	hex = strings.TrimPrefix(hex, "#")
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return color.NRGBA{R: r, G: g, B: b, A: 0xff}
}

type overlayTheme struct{}

func (t *overlayTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}
func (t *overlayTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}
func (t *overlayTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (t *overlayTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 13
	}
	if name == theme.SizeNamePadding {
		return 4
	}
	return theme.DefaultTheme().Size(name)
}
