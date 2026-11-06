package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8291A8"))
	accent   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5EEAD4"))
	gold     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	red      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185"))
	bold     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0"))
	selected = lipgloss.NewStyle().Foreground(lipgloss.Color("#5EEAD4")).Background(lipgloss.Color("#19313C"))
)

func cell(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func state(s string) string {
	if s == "baseline" || s == "blessed" || s == "ok" || s == "current" {
		return accent.Render(s)
	}
	if strings.HasPrefix(s, "differs") || strings.HasPrefix(s, "behind") || s == "pending" || s == "unpinned" {
		return gold.Render(s)
	}
	if s == "drift" || s == "unreachable" {
		return red.Render(s)
	}
	return muted.Render(s)
}
func (m model) selectedCount() int {
	n := 0
	for _, id := range m.visible() {
		if m.chosen[m.hosts[id].Name] {
			n++
		}
	}
	return n
}
func (m model) fleet(w int) []string {
	ids := m.visible()
	start, end := m.bounds(len(ids))
	query := m.query
	if query == "" {
		query = "all hosts · / to search"
	}
	if m.searching {
		query += "█"
	}
	rows := []string{muted.Render("FILTER ") + query + muted.Render(fmt.Sprintf("   %d matches · %d selected in filter", len(ids), m.selectedCount()))}
	nameWidth := max(18, min(32, w/4))
	appWidth := max(12, min(23, w/6))
	systemWidth := 14
	format := func(name, product, apps, system, server, repo, version string) string {
		s := cell(name, nameWidth) + " " + cell(apps, appWidth) + " " + cell(system, systemWidth) + " " + cell(server, 11) + " " + cell(repo, 8)
		if w >= 115 {
			s += " " + cell(product, 9)
		}
		if w >= 95 {
			s += " " + cell(version, 18)
		}
		return s
	}
	rows = append(rows, muted.Render(format("   HOST", "PRODUCT", "APPS", "SYSTEM", "SERVER", "REPO", "APP VERSION")), muted.Render(strings.Repeat("─", w)))
	for pos := start; pos < end; pos++ {
		h := m.hosts[ids[pos]]
		pointer := " "
		if pos == m.cursor {
			pointer = "›"
		}
		check := " "
		if m.chosen[h.Name] {
			check = "✓"
		}
		version := "-"
		if len(h.Instances) == 1 {
			version = h.Instances[0].Line + "@" + short(h.Instances[0].Repos["darcee"])
		} else if len(h.Instances) > 1 {
			version = fmt.Sprintf("%d instances", len(h.Instances))
		}
		line := format(pointer+check+" "+h.Name, h.Product, state(m.apps(h)), state(m.system(h)), state(h.Server), state(m.repoState(h)), version)
		if pos == m.cursor {
			line = selected.Render(line)
		}
		rows = append(rows, line)
	}
	if len(ids) == 0 {
		rows = append(rows, muted.Render("No hosts match. Esc clears the filter."))
	}
	rows = append(rows, muted.Render(fmt.Sprintf("Showing %d–%d / %d  ·  PgUp/PgDn page  ·  x expand to 120 mock hosts", min(start+1, len(ids)), end, len(ids))))
	if id := m.hostIndex(); id >= 0 {
		h := m.hosts[id]
		rows = append(rows, bold.Render(h.Name)+muted.Render("  "+h.Target+"  · running "+short(h.Running)+" → repo "+short(h.Revision)+" · "+h.LastDeploy))
	}
	return rows
}
func (m model) detail(w int) []string {
	id := m.hostIndex()
	if id < 0 {
		return []string{"No matching host. Esc back."}
	}
	h := m.hosts[id]
	rows := []string{bold.Render(h.Name) + "  " + state(h.Server) + "  " + state(m.repoState(h)), "",
		muted.Render("Target  ") + h.Target + muted.Render("  · last deployment ") + h.LastDeploy,
		"System  " + short(h.System) + " → baseline " + short(m.baseline) + "  " + state(m.system(h)),
		"Config  " + short(h.Running) + " running → " + short(h.Revision) + " in repo  (separate from app commits)",
		"", accent.Bold(true).Render("INSTANCES") + muted.Render("  · i switches focused instance"), ""}
	if len(h.Instances) == 0 {
		return append(rows, "DB-only host · PostgreSQL + PostGIS; no application pins.")
	}
	for i, in := range h.Instances {
		pointer := "  "
		if i == m.instanceIndex%len(h.Instances) {
			pointer = "› "
		}
		rows = append(rows, pointer+in.Name+"  product="+in.Product+"  line="+in.Line)
	}
	in := h.Instances[m.instanceIndex%len(h.Instances)]
	l := m.line(in.Line)
	rows = append(rows, "", muted.Render("  REPOSITORY       HOST PIN    BLESSED     STATE"))
	for _, repo := range m.repos() {
		status := "blessed"
		if in.Repos[repo] != l.Repos[repo] {
			status = "differs"
		}
		rows = append(rows, "  "+cell(repo, 16)+" "+cell(short(in.Repos[repo]), 11)+cell(short(l.Repos[repo]), 12)+state(status))
	}
	return rows
}
func (m model) releases(w int) []string {
	rows := []string{bold.Render("RELEASE LINES"), muted.Render("Stable → v42 · hosts adopt blessed pins with Update apps"), "", muted.Render("  LINE       BRANCH        PRODUCT    DARCEE    CHECKS")}
	for i, l := range m.lines {
		s := "  " + cell(l.Name, 10) + " " + cell(l.Branch, 13) + " " + cell(l.Product, 10) + " " + cell(short(l.Repos["darcee"]), 9) + l.Tested
		if i == m.cursor {
			s = selected.Render("›" + s[1:])
		}
		rows = append(rows, s)
	}
	return append(rows, "", accent.Bold(true).Render("BLESS A LINE"), "b resolve branch heads → VM checks → commit blessed pins", "", muted.Render("Service/VM checks block blessing. Unit test failures are reported only."))
}
func (m model) baselineView(w int) []string {
	behind := 0
	for _, h := range m.hosts {
		if h.System != m.baseline {
			behind++
		}
	}
	return []string{bold.Render("SYSTEM BASELINE"), muted.Render("Independent of application release lines"), "",
		"Channel   github:NixOS/nixpkgs/nixos-26.05", "Revision  " + accent.Render(short(m.baseline)), "Checks    " + m.baselineTest,
		fmt.Sprintf("Coverage  %d baseline / %d behind or unpinned", len(m.hosts)-behind, behind),
		"", accent.Bold(true).Render("ROLLOUT"),
		"b prepare new baseline → seeded allinone VM → report unit tests → commit",
		"Fleet: filter hosts → A select matches → p adopt baseline → d deploy",
		"", gold.Render("Patch system preserves all darcee, evvue, and extension pins."),
		muted.Render("Changes remain pending until deployed; shared profiles affect future deploys.")}
}
func (m model) vmView(w int) []string {
	status := "stopped"
	if m.vmRunning {
		status = "running"
	}
	maintenance := "off"
	if m.maintenance {
		maintenance = "on · HTTP 503 / HTML + JSON"
	}
	return []string{bold.Render("LOCAL VM / allinone") + "  " + accent.Render(status), "",
		"App          http://localhost:8080   (mock)", "Shell        root@localhost:2222    (mock)", "Disk         persistent across mock starts", "Maintenance  " + maintenance,
		"", accent.Bold(true).Render("VM CONTROLS"), "s start VM   m toggle maintenance   l show mock logs   R reset disk", "",
		muted.Render("Reset is reviewed before clearing the in-memory mock VM state.")}
}
func (m model) activity() []string {
	start, end := m.bounds(len(m.events))
	rows := []string{bold.Render("ACTIVITY"), muted.Render("Current session · ↑↓ / PgUp/PgDn scroll"), ""}
	for i := start; i < end; i++ {
		e := m.events[i]
		rows = append(rows, muted.Render(e.When)+"  "+e.Text)
	}
	return rows
}
func (m model) confirmation() []string {
	rows := []string{bold.Render("REVIEW / " + m.action), ""}
	if len(m.targets) > 0 {
		rows = append(rows, fmt.Sprintf("Scope   %d host(s) · actions apply to selected matches in current filter", len(m.targets)))
		for i, id := range m.targets {
			if i >= 3 {
				rows = append(rows, fmt.Sprintf("        + %d more", len(m.targets)-3))
				break
			}
			h := m.hosts[id]
			rows = append(rows, "        "+h.Name+" → "+h.Target)
		}
		rows = append(rows, "")
	}
	switch m.action {
	case "Update apps":
		source := "blessed pins"
		if m.head {
			source = "branch heads (untested)"
		}
		rows = append(rows, "Source  "+source+"  · H toggles branch heads", "Effect  update every app instance; clear holds; preserve system pins")
	case "Patch system":
		rows = append(rows, "System  adopt baseline "+short(m.baseline), "Effect  preserve every application repository pin")
	case "Hold repo":
		h := m.hosts[m.targets[0]]
		in := h.Instances[m.instanceIndex%len(h.Instances)]
		repo := m.repos()[m.repoIndex%len(m.repos())]
		rows = append(rows, "Instance "+in.Name+" / "+in.Product+" / "+in.Line, "Repo     "+repo+" · current "+short(in.Repos[repo])+"  ([ / ] change repository)", "Revision "+m.input+"█  (hex commit or ~N; Backspace to edit)", "Effect   hold this repo until the next Update apps")
	case "Deploy":
		mode := "switch + verify + deployment tag"
		if m.buildOnly {
			mode = "build only; no activation or tag"
		}
		rows = append(rows, "Mode    "+mode+"  · B toggles build only", "State   complete configuration: application pins + system pins")
	case "Bless line":
		l := m.lines[m.cursor]
		rows = append(rows, "Line    "+l.Name+" / "+l.Branch+" / test product "+l.Product)
	case "Baseline":
		rows = append(rows, "System  nixos-26.05 head → new tested baseline; hosts unchanged")
	case "VM reset":
		rows = append(rows, gold.Render("Reset the mock disk and stop the mock VM; next start is fresh."))
	}
	if m.action == "Baseline" || m.action == "Bless line" {
		tests := "VM checks enabled"
		if m.skipTests {
			tests = "skip tests; mark result untested"
		}
		rows = append(rows, "", "Tests   "+tests+"  · T toggles --no-test", "Report  application unit failures do not block blessing")
	}
	if len(m.targets) > 1 {
		rows = append(rows, muted.Render("Sequential queue; failure stops it. Completed targets retain their changes."))
	}
	rows = append(rows, "", accent.Render("SIMULATION · in-memory state only"))
	return rows
}
func (m model) operation() []string {
	subject := "global"
	if len(m.targets) > 0 {
		id := m.targets[min(m.operationHost, len(m.targets)-1)]
		subject = fmt.Sprintf("%s · host %d/%d", m.hosts[id].Name, min(m.operationHost+1, len(m.targets)), len(m.targets))
	}
	rows := []string{bold.Render(strings.ToUpper(m.action)), muted.Render(subject), ""}
	for i, s := range m.steps() {
		prefix := muted.Render("○")
		if i < m.step {
			prefix = accent.Render("✓")
		} else if i == m.step {
			if m.failed {
				prefix = red.Render("×")
			} else {
				prefix = accent.Render([]string{"⠋", "⠙", "⠹", "⠸"}[m.frame%4])
			}
		}
		rows = append(rows, prefix+" "+s)
	}
	total := len(m.steps()) * max(1, len(m.targets))
	done := m.step
	if len(m.targets) > 0 {
		done = m.operationHost*len(m.steps()) + m.step
		if !m.busy && !m.failed {
			done = total
		}
	}
	pct := min(100, done*100/total)
	fill := pct * 30 / 100
	rows = append(rows, "", accent.Render(strings.Repeat("━", fill))+muted.Render(strings.Repeat("─", 30-fill))+fmt.Sprintf(" %d%%", pct), "", accent.Bold(true).Render("MOCK OUTPUT"))
	for _, s := range m.logs[max(0, len(m.logs)-3):] {
		rows = append(rows, muted.Render("› ")+s)
	}
	if m.busy {
		rows = append(rows, muted.Render("Timers running · esc cancels remaining work"))
	} else if m.failed {
		rows = append(rows, red.Render("Failed · current target unchanged; earlier targets retained."), "enter return")
	} else {
		rows = append(rows, accent.Render("✓ Complete · enter return"))
	}
	return rows
}
func (m model) View() string {
	if m.width < 76 || m.height < 22 {
		return "Octopus · compact prototype\nUse a terminal at least 76 × 22. q quits.\n"
	}
	w := m.width - 2
	tabs := []string{}
	for i, name := range []string{"1 Fleet", "2 Lines", "3 System", "4 VM", "5 Activity"} {
		if m.tab == i {
			tabs = append(tabs, selected.Render(name))
		} else {
			tabs = append(tabs, muted.Render(name))
		}
	}
	title := accent.Bold(true).Render("◈ OCTOPUS") + "  " + strings.Join(tabs, "  ") + gold.Render("  MOCK")
	pending, behind, apps := 0, 0, 0
	for _, h := range m.hosts {
		if m.repoState(h) != "current" {
			pending++
		}
		if m.system(h) != "baseline" {
			behind++
		}
		if s := m.apps(h); s != "blessed" && s != "-" {
			apps++
		}
	}
	summary := muted.Render(fmt.Sprintf("%d hosts · %d app differences · %d system patches · %d undeployed", len(m.hosts), apps, behind, pending))
	fleet := m.page == "dashboard" && m.tab == 0
	rows := []string{title, summary}
	if !fleet {
		rows = []string{title, muted.Render(strings.Repeat("─", w)), ""}
	}
	var body []string
	switch m.page {
	case "detail":
		body = m.detail(w)
	case "confirm":
		body = m.confirmation()
	case "run":
		body = m.operation()
	default:
		switch m.tab {
		case 0:
			body = m.fleet(w)
		case 1:
			body = m.releases(w)
		case 2:
			body = m.baselineView(w)
		case 3:
			body = m.vmView(w)
		case 4:
			body = m.activity()
		}
	}
	foot := "↑↓ move  / search  space select  A all  c clear  enter details"
	actions := "u apps  p system  d deploy  o hold  x 120 hosts  f fail  r reset  q quit"
	if m.tab != 0 {
		foot = "tab / 1–5 switch views  ↑↓ move  PgUp/PgDn page"
		actions = "b bless line / baseline  f toggle failure  r reset  q quit"
	}
	if m.tab == 3 {
		actions = "s start  m maintenance  l logs  R reset disk  f fail  q quit"
	}
	if m.page == "detail" {
		foot = "i next instance  u apps  p system  d deploy  o hold  esc back"
	}
	if m.page == "confirm" {
		foot = "enter simulate  esc back"
		actions = "All actions use timers; no subprocesses or network calls."
	}
	if m.page == "run" {
		foot = "esc cancel remaining work  enter return when finished  q quit"
		actions = "Completed targets persist in this session; r resets demo on dashboard."
	}
	if m.searching {
		foot = "Type to filter name / product / line / apps / system / server / repo"
		actions = "enter apply  esc clear filter"
	}
	if m.fail {
		actions = gold.Render("FAILURE MODE · step 3  ") + actions
	}
	footer := []string{muted.Render(foot), muted.Render(actions)}
	if m.notice != "" {
		footer = append([]string{gold.Render(m.notice)}, footer...)
	}
	if !fleet {
		footer = append([]string{"", muted.Render(strings.Repeat("─", w))}, footer...)
		// Collapse optional section gaps only when a small terminal needs the rows.
		// Header separation and the footer divider always stay visible.
		for len(rows)+len(body)+len(footer) > m.height {
			gap := -1
			for i := len(body) - 1; i >= 0; i-- {
				if body[i] == "" {
					gap = i
					break
				}
			}
			if gap < 0 {
				break
			}
			body = append(body[:gap], body[gap+1:]...)
		}
	}
	rows = append(rows, body...)
	rows = append(rows, footer...)
	// Never wrap rows: dense tables keep the same vertical footprint at any width.
	for i, s := range rows {
		rows[i] = " " + ansi.Truncate(s, w, "…")
	}
	return strings.Join(rows, "\n")
}
