package main

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	modeList = iota
	modeForm
	modeLog
	modeHelp
	modeConfirm
)

const (
	fName = iota
	fType
	fHost
	fLocalPort
	fRemoteHost
	fRemotePort
	fSave
	fCount
)

var labels = [fCount]string{"Name", "Type", "SSH host", "Local port", "Remote host", "Remote port", "Save"}

var (
	accent = lipgloss.AdaptiveColor{Light: "26", Dark: "75"}
	selBg  = lipgloss.AdaptiveColor{Light: "254", Dark: "237"}

	plain  = lipgloss.NewStyle()
	bold   = lipgloss.NewStyle().Bold(true)
	hi     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	dim    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "246"})
	faint  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "240"})
	green  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "78"})
	yellow = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "136", Dark: "221"})
	red    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"})

	badge  = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(accent).Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "232"})
	button = lipgloss.NewStyle().Padding(0, 2).Background(selBg).Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "246"})
	box    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.AdaptiveColor{Light: "250", Dark: "240"}).Padding(1, 3).MarginLeft(2)
)

var helpKeys = [][2]string{
	{"↑ ↓  k j", "move the selection"},
	{"enter  space", "start or stop the selected tunnel"},
	{"a", "add a tunnel"},
	{"e", "edit the selected tunnel (stops it on save)"},
	{"d", "delete the selected tunnel"},
	{"c", "duplicate the selected tunnel"},
	{"l", "show the ssh log of the selected tunnel"},
	{"X", "stop all tunnels"},
	{"q  ctrl+c", "quit; running tunnels keep running"},
}

var placeholders = [fCount]string{"prod-db", "", "alias or user@host", "5432", "localhost", "same as local port", ""}

type tickMsg time.Time

type trustedMsg string

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	tunnels []Tunnel
	runs    map[string]run
	up      map[string]bool
	errs    map[string]string
	cursor  int
	mode    int
	width   int
	height  int
	notice  string

	inputs  [fCount]textinput.Model
	typ     int
	focus   int
	editing int
	formErr string
	hosts   []string
	hostSel int

	question string
	onYes    func()

	logName string
	logText string
}

func newModel(tunnels []Tunnel, runs map[string]run) *model {
	m := &model{tunnels: tunnels, runs: runs, up: map[string]bool{}, errs: map[string]string{}}
	for name, r := range runs {
		if m.find(name) < 0 {
			stop(r)
			delete(runs, name)
		}
	}
	m.refresh(true)
	m.saveRuns()
	return m
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) find(name string) int {
	return slices.IndexFunc(m.tunnels, func(t Tunnel) bool { return t.Name == name })
}

func (m *model) saveRuns() {
	if err := saveJSON("state.json", m.runs); err != nil {
		m.notice = err.Error()
	}
}

func (m *model) saveTunnels() {
	if err := saveJSON("tunnels.json", config{m.tunnels}); err != nil {
		m.notice = err.Error()
	}
}

func (m *model) refresh(first bool) {
	changed := false
	for name, r := range m.runs {
		if sshAlive(r.PID) {
			if !m.up[name] && up(name) {
				m.up[name] = true
			}
			continue
		}
		if m.up[name] {
			m.errs[name] = "connection lost"
		} else if !first {
			m.errs[name] = failReason(readLog(name), m.tunnels[m.find(name)])
		}
		delete(m.runs, name)
		delete(m.up, name)
		changed = true
	}
	if changed {
		m.saveRuns()
	}
}

func (m *model) toggle(t Tunnel) {
	if _, running := m.runs[t.Name]; running {
		m.stopTunnel(t.Name)
		return
	}
	delete(m.errs, t.Name)
	r, err := start(t)
	if err != nil {
		m.errs[t.Name] = err.Error()
		return
	}
	m.runs[t.Name] = r
	m.saveRuns()
}

func (m *model) stopTunnel(name string) {
	if r, ok := m.runs[name]; ok {
		stop(r)
		delete(m.runs, name)
		delete(m.up, name)
		m.saveRuns()
	}
	delete(m.errs, name)
}

func (m *model) ask(question string, onYes func()) {
	m.question, m.onYes, m.mode = question, onYes, modeConfirm
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.refresh(false)
		if m.mode == modeLog {
			m.logText = readLog(m.logName)
		}
		return m, tick()
	case trustedMsg:
		if i := m.find(string(msg)); i >= 0 {
			m.toggle(m.tunnels[i])
		}
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeForm:
		m.formKey(k)
		return m, nil
	case modeConfirm:
		m.mode = modeList
		if s == "y" {
			m.onYes()
		}
		return m, nil
	case modeLog, modeHelp:
		m.mode = modeList
		return m, nil
	}

	m.notice = ""
	switch s {
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.cursor = max(m.cursor-1, 0)
	case "down", "j":
		m.cursor = max(min(m.cursor+1, len(m.tunnels)-1), 0)
	case "a":
		m.openForm(Tunnel{Type: "local", RemoteHost: "localhost"}, -1)
	case "?":
		m.mode = modeHelp
	case "X":
		m.ask("Stop all tunnels?", func() {
			for name := range m.runs {
				m.stopTunnel(name)
			}
		})
	}
	if m.cursor >= len(m.tunnels) {
		return m, nil
	}

	i, t := m.cursor, m.tunnels[m.cursor]
	switch s {
	case "enter", " ":
		if m.errs[t.Name] == errHostKey {
			return m, tea.ExecProcess(trustCmd(t), func(error) tea.Msg { return trustedMsg(t.Name) })
		}
		m.toggle(t)
	case "e":
		m.openForm(t, i)
	case "c":
		t.Name += "-copy"
		m.openForm(t, -1)
	case "l":
		m.logName, m.logText, m.mode = t.Name, readLog(t.Name), modeLog
	case "d":
		m.ask("Delete "+t.Name+"?", func() {
			m.stopTunnel(t.Name)
			os.Remove(logPath(t.Name))
			m.tunnels = slices.Delete(m.tunnels, i, i+1)
			m.cursor = max(min(m.cursor, len(m.tunnels)-1), 0)
			m.saveTunnels()
		})
	}
	return m, nil
}

func (m *model) openForm(t Tunnel, editing int) {
	port := func(p int) string {
		if p == 0 {
			return ""
		}
		return strconv.Itoa(p)
	}
	values := [fCount]string{t.Name, "", t.Host, port(t.LocalPort), t.RemoteHost, port(t.RemotePort), ""}
	for i := range m.inputs {
		in := textinput.New()
		in.Prompt = ""
		in.CharLimit = 64
		in.Width = 40
		in.Placeholder = placeholders[i]
		in.PlaceholderStyle = faint
		in.Cursor.Style = lipgloss.NewStyle().Foreground(accent)
		in.Cursor.SetMode(cursor.CursorStatic)
		in.SetValue(values[i])
		m.inputs[i] = in
	}
	m.hosts = sshHosts()
	m.typ = max(slices.Index(types, t.Type), 0)
	m.editing, m.formErr, m.mode = editing, "", modeForm
	m.setFocus(fName)
}

func (m *model) fields() []int {
	if types[m.typ] == "dynamic" {
		return []int{fName, fType, fHost, fLocalPort, fSave}
	}
	return []int{fName, fType, fHost, fLocalPort, fRemoteHost, fRemotePort, fSave}
}

func (m *model) setFocus(f int) {
	m.inputs[m.focus].Blur()
	m.focus = f
	m.hostSel = -1
	m.inputs[f].Focus()
}

func (m *model) hostMatches() []string {
	typed := strings.ToLower(m.inputs[fHost].Value())
	var out []string
	for _, h := range m.hosts {
		if strings.Contains(strings.ToLower(h), typed) {
			out = append(out, h)
		}
	}
	return out
}

func (m *model) step(delta int) {
	fields := m.fields()
	i := slices.Index(fields, m.focus) + delta
	m.setFocus(fields[(i+len(fields))%len(fields)])
}

func (m *model) formKey(k tea.KeyMsg) {
	s := k.String()
	onHost := m.focus == fHost
	matches := m.hostMatches()
	if onHost && m.hostSel >= 0 && (s == "enter" || s == "tab") {
		m.inputs[fHost].SetValue(matches[m.hostSel])
	}
	switch {
	case s == "esc":
		m.mode = modeList
	case s == "enter" && m.focus == fSave:
		m.save()
	case onHost && s == "down" && m.hostSel < len(matches)-1:
		m.hostSel++
	case onHost && s == "up" && m.hostSel >= 0:
		m.hostSel--
	case s == "tab", s == "enter", s == "down":
		m.step(1)
	case s == "shift+tab", s == "up":
		m.step(-1)
	case m.focus == fType:
		switch s {
		case "left":
			m.typ = (m.typ + len(types) - 1) % len(types)
		case "right", " ":
			m.typ = (m.typ + 1) % len(types)
		}
	default:
		m.inputs[m.focus], _ = m.inputs[m.focus].Update(k)
		m.hostSel = -1
	}
}

func (m *model) formTunnel() Tunnel {
	val := func(f int) string { return strings.TrimSpace(m.inputs[f].Value()) }
	t := Tunnel{Name: val(fName), Type: types[m.typ], Host: val(fHost)}
	t.LocalPort, _ = strconv.Atoi(val(fLocalPort))
	if t.Type != "dynamic" {
		t.RemoteHost = val(fRemoteHost)
		t.RemotePort = t.LocalPort
		if v := val(fRemotePort); v != "" {
			t.RemotePort, _ = strconv.Atoi(v)
		}
	}
	return t
}

func (m *model) save() {
	t := m.formTunnel()
	if err := t.validate(); err != nil {
		m.formErr = err.Error()
		return
	}
	if i := m.find(t.Name); i >= 0 && i != m.editing {
		m.formErr = "name: already used"
		return
	}
	if m.editing >= 0 {
		old := m.tunnels[m.editing]
		m.stopTunnel(old.Name)
		if old.Name != t.Name {
			os.Remove(logPath(old.Name))
		}
		m.tunnels[m.editing] = t
	} else {
		m.tunnels = append(m.tunnels, t)
		m.cursor = len(m.tunnels) - 1
	}
	m.saveTunnels()
	m.mode = modeList
}

func (m *model) View() string {
	var s string
	switch m.mode {
	case modeForm:
		s = m.formView()
	case modeLog:
		s = m.logView()
	case modeHelp:
		s = m.helpView()
	default:
		s = m.listView()
	}
	if m.width > 0 {
		s = lipgloss.NewStyle().MaxWidth(m.width).Render(s)
	}
	return s
}

func hints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, hi.Render(pairs[i])+" "+dim.Render(pairs[i+1]))
	}
	return strings.Join(parts, "  ")
}

func padTo(s string, w int) string {
	return s + strings.Repeat(" ", max(w-lipgloss.Width(s), 0))
}

func (m *model) status(t Tunnel) (lipgloss.Style, string) {
	if e, ok := m.errs[t.Name]; ok {
		if e == errHostKey {
			e += " · enter to review"
		}
		return red, "✕ " + e
	}
	r, running := m.runs[t.Name]
	switch {
	case !running:
		return dim, "○ stopped"
	case m.up[t.Name]:
		return green, "● up " + uptime(time.Since(r.Started))
	}
	return yellow, "◐ connecting"
}

func (m *model) listView() string {
	var b strings.Builder

	errStyle := dim
	if len(m.errs) > 0 {
		errStyle = red
	}
	sep := faint.Render(" · ")
	b.WriteString("\n  " + badge.Render("fwdhub") + "  " +
		green.Render(fmt.Sprintf("%d up", len(m.up))) + sep +
		errStyle.Render(fmt.Sprintf("%d error", len(m.errs))) + sep +
		dim.Render(fmt.Sprintf("%d total", len(m.tunnels))) + "\n\n")

	if len(m.tunnels) == 0 {
		b.WriteString("  " + dim.Render("No tunnels yet. Press ") + hi.Render("a") + dim.Render(" to add one.") + "\n")
	} else {
		b.WriteString(m.table())
	}

	b.WriteString("\n")
	if m.notice != "" {
		b.WriteString("  " + red.Render("✕ "+m.notice) + "\n")
	}
	if m.mode == modeConfirm {
		b.WriteString("  " + yellow.Bold(true).Render(m.question) + "  " + hints("y", "yes", "n", "no"))
	} else {
		b.WriteString("  " + hints("enter", "start/stop", "a", "add", "e", "edit", "d", "delete", "l", "log", "X", "stop all", "?", "help", "q", "quit"))
	}
	return b.String()
}

func (m *model) table() string {
	const gap = 3
	styles := []lipgloss.Style{bold, dim, plain, hi, plain, dim}
	rows := [][]string{{"NAME", "TYPE", "LOCAL", "", "REMOTE", "VIA"}}
	statuses := []string{"STATUS"}
	statusStyles := []lipgloss.Style{dim}
	for _, t := range m.tunnels {
		arrow, remote := "→", fmt.Sprintf("%s:%d", t.RemoteHost, t.RemotePort)
		switch t.Type {
		case "remote":
			arrow = "←"
		case "dynamic":
			remote = "SOCKS proxy"
		}
		rows = append(rows, []string{t.Name, t.Type, fmt.Sprintf("localhost:%d", t.LocalPort), arrow, remote, t.Host})
		style, text := m.status(t)
		statuses = append(statuses, text)
		statusStyles = append(statusStyles, style)
	}

	widths := make([]int, len(rows[0]))
	statusWidth := 0
	for r, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], lipgloss.Width(cell))
		}
		statusWidth = max(statusWidth, lipgloss.Width(statuses[r]))
	}

	line := func(r int, styles []lipgloss.Style, on func(lipgloss.Style) lipgloss.Style) string {
		var s string
		for i, cell := range rows[r] {
			s += on(styles[i]).Render(padTo(cell, widths[i]+gap))
		}
		return s + on(statusStyles[r]).Render(padTo(statuses[r], statusWidth+1))
	}
	same := func(s lipgloss.Style) lipgloss.Style { return s }
	selected := func(s lipgloss.Style) lipgloss.Style { return s.Background(selBg) }
	header := []lipgloss.Style{dim, dim, dim, dim, dim, dim}

	var b strings.Builder
	b.WriteString("    " + line(0, header, same) + "\n")
	b.WriteString("  " + faint.Render(strings.Repeat("─", lipgloss.Width(line(0, header, same))+2)) + "\n")
	for i := range m.tunnels {
		if i == m.cursor {
			b.WriteString("  " + selected(hi).Render("▌ ") + line(i+1, styles, selected) + "\n")
		} else {
			b.WriteString("    " + line(i+1, styles, same) + "\n")
		}
	}
	return b.String()
}

func (m *model) formView() string {
	var b strings.Builder
	title := "Add tunnel"
	if m.editing >= 0 {
		title = "Edit tunnel"
	}
	b.WriteString("  " + hi.Render(title) + "\n\n")

	const labelWidth = 14
	for _, f := range m.fields() {
		if f == fSave {
			continue
		}
		label := padTo(labels[f], labelWidth)
		if f == m.focus {
			b.WriteString(hi.Render("▌ " + label))
		} else {
			b.WriteString("  " + dim.Render(label))
		}
		if f != fType {
			b.WriteString(m.inputs[f].View() + "\n")
			if f == fHost && m.focus == fHost {
				b.WriteString(m.hostList(labelWidth))
			}
			continue
		}
		for i, t := range types {
			name := strings.ToUpper(t[:1]) + t[1:]
			if i == m.typ {
				b.WriteString(hi.Render("● "+name) + "   ")
			} else {
				b.WriteString(dim.Render("○ "+name) + "   ")
			}
		}
		b.WriteString("\n")
	}

	if m.formErr != "" {
		b.WriteString("\n  " + red.Render("✕ "+m.formErr) + "\n")
	}
	if m.focus == fSave {
		b.WriteString("\n" + hi.Render("▌ ") + badge.Padding(0, 2).Render("Save") + "\n")
	} else {
		b.WriteString("\n  " + button.Render("Save") + "\n")
	}
	t := m.formTunnel()
	b.WriteString("\n  " + faint.Render("$ ") + dim.Render("ssh -N "+strings.Join(t.forward(), " ")+" "+t.Host))

	return "\n" + box.Render(b.String()) + "\n\n  " +
		hints("tab", "next", "←/→", "type", "↑/↓", "pick host", "enter", "on Save", "esc", "cancel")
}

func (m *model) hostList(indent int) string {
	const rows = 8
	matches := m.hostMatches()
	start := max(m.hostSel-rows+1, 0)
	end := min(start+rows, len(matches))
	pad := strings.Repeat(" ", indent)

	var b strings.Builder
	for i := start; i < end; i++ {
		if i == m.hostSel {
			b.WriteString(pad + hi.Render("› "+matches[i]) + "\n")
		} else {
			b.WriteString(pad + dim.Render("  "+matches[i]) + "\n")
		}
	}
	if end < len(matches) {
		b.WriteString(pad + faint.Render(fmt.Sprintf("  … %d more", len(matches)-end)) + "\n")
	}
	return b.String()
}

func (m *model) helpView() string {
	var b strings.Builder
	b.WriteString(hi.Render("Keys") + "\n\n")
	for _, k := range helpKeys {
		b.WriteString(hi.Render(padTo(k[0], 15)) + k[1] + "\n")
	}
	return "\n" + box.Render(strings.TrimRight(b.String(), "\n")) + "\n\n  " + hints("any key", "back")
}

func (m *model) logView() string {
	body := "  " + dim.Render("No log yet.")
	if text := strings.TrimSpace(m.logText); text != "" {
		lines := strings.Split(text, "\n")
		if n := m.height - 7; n > 0 && len(lines) > n {
			lines = lines[len(lines)-n:]
		}
		body = "  " + strings.Join(lines, "\n  ")
	}
	return "\n  " + badge.Render("log") + "  " + bold.Render(m.logName) + "\n\n" + body + "\n\n  " + hints("any key", "back")
}
