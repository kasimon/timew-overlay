package main

import (
	"bytes"
	"fmt"
	"image/color"
	"os/exec"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type tableRow struct {
	cols []string
}

var (
	detailRows   []tableRow
	summaryRows  []tableRow
	dataMu       sync.Mutex
)

var (
	colorRowEven   = color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff}
	colorRowOdd    = color.NRGBA{R: 0xdd, G: 0xdd, B: 0xdd, A: 0xff}
	colorRowHeader = color.NRGBA{R: 0xcc, G: 0xcc, B: 0xcc, A: 0xff}
	colorTextDark  = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
)

const (
	viewDetails = iota
	viewSummary
)

func makeTable(header []string, data *[]tableRow, colWidths []float32) *widget.Table {
	ncols := len(header)
	t := widget.NewTable(
		func() (int, int) {
			dataMu.Lock()
			defer dataMu.Unlock()
			return len(*data) + 1, ncols
		},
		func() fyne.CanvasObject {
			bg := canvas.NewRectangle(colorRowEven)
			txt := canvas.NewText("", colorTextDark)
			txt.TextSize = 13
			return container.NewStack(bg, container.NewPadded(txt))
		},
		func(id widget.TableCellID, cell fyne.CanvasObject) {
			stack := cell.(*fyne.Container)
			bg := stack.Objects[0].(*canvas.Rectangle)
			txt := stack.Objects[1].(*fyne.Container).Objects[0].(*canvas.Text)

			txt.Color = colorTextDark
			txt.TextStyle = fyne.TextStyle{}

			if id.Row == 0 {
				bg.FillColor = colorRowHeader
				txt.TextStyle = fyne.TextStyle{Bold: true}
				if id.Col < len(header) {
					txt.Text = header[id.Col]
				}
				bg.Refresh()
				txt.Refresh()
				return
			}

			dataMu.Lock()
			idx := id.Row - 1
			if idx >= len(*data) {
				dataMu.Unlock()
				return
			}
			row := (*data)[idx]
			dataMu.Unlock()

			if id.Row%2 == 0 {
				bg.FillColor = colorRowEven
			} else {
				bg.FillColor = colorRowOdd
			}
			bg.Refresh()

			if id.Col < len(row.cols) {
				txt.Text = row.cols[id.Col]
			} else {
				txt.Text = ""
			}
			txt.Refresh()
		},
	)
	for i, w := range colWidths {
		t.SetColumnWidth(i, w)
	}
	return t
}

func main() {
	a := app.New()
	a.Settings().SetTheme(&overlayTheme{})
	w := a.NewWindow("timew")

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
	tableView := viewDetails

	detailTable := makeTable([]string{"Tag", "Period", "Dur"}, &detailRows, []float32{100, 120, 55})
	summaryTable := makeTable([]string{"Tag", "Total"}, &summaryRows, []float32{200, 60})

	detailContainer := container.NewStack(detailTable)
	summaryContainer := container.NewStack(summaryTable)
	summaryContainer.Hide()

	tableStack := container.NewStack(detailContainer, summaryContainer)

	activeTable := func() *widget.Table {
		if tableView == viewDetails {
			return detailTable
		}
		return summaryTable
	}

	activeRows := func() *[]tableRow {
		if tableView == viewDetails {
			return &detailRows
		}
		return &summaryRows
	}

	viewToggle := widget.NewCheck("Summarize by Tag", func(checked bool) {
		if checked {
			tableView = viewSummary
			detailContainer.Hide()
			summaryContainer.Show()
		} else {
			tableView = viewDetails
			summaryContainer.Hide()
			detailContainer.Show()
		}
	})

	viewToggleRow := container.NewHBox(viewToggle, layout.NewSpacer())
	tablePanel := container.NewVBox(viewToggleRow, tableStack)
	tablePanel.Hide()
	panelShown := false

	resizeWindow := func() {
		if panelShown {
			dataMu.Lock()
			rows := len(*activeRows())
			dataMu.Unlock()
			rowH := float32(30)
			tableH := float32(rows+1)*rowH + 4
			if tableH > 400 {
				tableH = 400
			}
			activeTable().Resize(fyne.NewSize(310, tableH))
			activeTable().Refresh()
			w.Resize(fyne.NewSize(340, 160+tableH))
		} else {
			w.Resize(fyne.NewSize(300, 120))
		}
	}

	// Wire up resize after view toggle
	origOnChanged := viewToggle.OnChanged
	viewToggle.OnChanged = func(checked bool) {
		origOnChanged(checked)
		updateAllTableData()
		activeTable().Refresh()
		resizeWindow()
	}

	toggleBtn = widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		if isTracking {
			runTimew("stop")
		} else {
			runTimew("start")
		}
		refreshStatus(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
		if panelShown {
			updateAllTableData()
			activeTable().Refresh()
			resizeWindow()
		}
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
		refreshStatus(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
		if panelShown {
			updateAllTableData()
			activeTable().Refresh()
			resizeWindow()
		}
	}

	expandBtn := widget.NewButton("[+]", func() {
		panelShown = !panelShown
		if panelShown {
			updateAllTableData()
			activeTable().Refresh()
			tablePanel.Show()
		} else {
			tablePanel.Hide()
		}
		resizeWindow()
	})

	refreshStatus(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(func() {
				refreshStatus(w, toggleBtn, statusIcon, statusLabel, dayTotal, elapsed, &isTracking)
				if panelShown {
					updateAllTableData()
					activeTable().Refresh()
					resizeWindow()
				}
			})
		}
	}()

	topRow := container.NewHBox(toggleBtn, statusIcon, statusLabel)
	bottomRow := container.NewHBox(dayTotal, layout.NewSpacer(), elapsed)
	tagRow := container.NewBorder(nil, nil, nil, expandBtn, tagEntry)

	content := container.NewVBox(
		topRow,
		bottomRow,
		widget.NewSeparator(),
		tagRow,
		tablePanel,
	)

	w.SetContent(container.NewPadded(content))
	w.Resize(fyne.NewSize(300, 120))
	w.SetOnClosed(func() { a.Quit() })
	w.ShowAndRun()
}

func refreshStatus(w fyne.Window, toggleBtn *widget.Button, icon *canvas.Text, status, dayTotal, elapsed *widget.Label, isTracking *bool) {
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
		elapsed.SetText(trimSeconds(elapsedStr))
		w.SetTitle(fmt.Sprintf("timew - %s (Active)", trimSeconds(total)))
	} else {
		icon.Text = "⏸"
		icon.Color = parseHexColor("#ff9800")
		toggleBtn.SetIcon(theme.MediaPlayIcon())
		status.SetText("Stopped")
		elapsed.SetText("")
		w.SetTitle(fmt.Sprintf("timew - %s (Stopped)", trimSeconds(total)))
	}
	icon.Refresh()
	dayTotal.SetText(fmt.Sprintf("Day: %s", trimSeconds(total)))
}

func updateAllTableData() {
	out := runTimew("summary", ":day")
	lines := strings.Split(strings.TrimSpace(out), "\n")

	var details []tableRow
	totals := make(map[string]time.Duration)
	var order []string

	if len(lines) >= 3 {
		for _, line := range lines[2:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) == 1 && isTimeDuration(fields[0]) {
				continue
			}
			tag, start, end, dur := parseSummaryLine(line)
			if start == "" {
				continue
			}

			// Details row
			if end == "-" {
				end = "now"
			} else {
				end = trimSeconds(end)
			}
			span := fmt.Sprintf("%s - %s", trimSeconds(start), end)
			details = append(details, tableRow{cols: []string{tag, span, trimSeconds(dur)}})

			// Summary accumulation
			tagKey := tag
			if tagKey == "" {
				tagKey = "(untagged)"
			}
			d := parseDuration(dur)
			if _, exists := totals[tagKey]; !exists {
				order = append(order, tagKey)
			}
			totals[tagKey] += d
		}
	}

	var summary []tableRow
	for _, tag := range order {
		summary = append(summary, tableRow{cols: []string{tag, formatDuration(totals[tag])}})
	}

	dataMu.Lock()
	detailRows = details
	summaryRows = summary
	dataMu.Unlock()
}

func parseDuration(s string) time.Duration {
	parts := strings.Split(s, ":")
	var h, m, sec int
	if len(parts) == 3 {
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &sec)
	} else if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%d:%02d", h, m)
}

func trimSeconds(t string) string {
	parts := strings.Split(t, ":")
	if len(parts) == 3 {
		// For durations, convert H:MM:SS to total minutes
		h := 0
		m := 0
		s := 0
		fmt.Sscanf(parts[0], "%d", &h)
		fmt.Sscanf(parts[1], "%d", &m)
		fmt.Sscanf(parts[2], "%d", &s)
		totalMin := h*60 + m
		if s >= 30 {
			totalMin++
		}
		return fmt.Sprintf("%d:%02d", totalMin/60, totalMin%60)
	}
	return t
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

func parseSummaryLine(line string) (tag, start, end, dur string) {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", "", "", ""
	}

	var times []string
	var nonTimes []string

	for _, f := range fields {
		if isTimeDuration(f) || f == "-" {
			times = append(times, f)
		} else if !isWeekNum(f) && !isDayName(f) && !isDate(f) {
			nonTimes = append(nonTimes, f)
		}
	}

	if len(times) >= 3 {
		start = times[0]
		end = times[1]
		dur = times[2]
	} else {
		return "", "", "", ""
	}

	tag = strings.Join(nonTimes, " ")
	return tag, start, end, dur
}

func isWeekNum(s string) bool {
	if len(s) != 3 || s[0] != 'W' {
		return false
	}
	return s[1] >= '0' && s[1] <= '9' && s[2] >= '0' && s[2] <= '9'
}

func isDayName(s string) bool {
	switch s {
	case "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun":
		return true
	}
	return false
}

func isDate(s string) bool {
	if len(s) != 10 {
		return false
	}
	return s[4] == '-' && s[7] == '-'
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
