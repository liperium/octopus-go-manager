package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"sort"
	"strings"
	"time"
)

// Repository-derived fixtures are compiled in. Runtime never reads the real fleet.
//
//go:embed hosts.json
var hostData []byte

//go:embed lines.json
var lineData []byte

type instance struct {
	Name, Product, Line string
	Repos               map[string]string
}
type host struct {
	Name, Product, System                         string
	Instances                                     []instance
	Target, Revision, Running, Server, LastDeploy string
}
type release struct {
	Name, Branch, Product string
	Repos                 map[string]string
	Tested                string
}
type event struct{ When, Text string }
type tickMsg struct{ Generation int }
type model struct {
	width, height, tab, cursor, instanceIndex, repoIndex      int
	hosts                                                     []host
	lines                                                     []release
	baseline, baselineTest                                    string
	events                                                    []event
	query, input, page, action, notice                        string
	searching, fail, busy, failed, buildOnly, head, skipTests bool
	chosen                                                    map[string]bool
	targets                                                   []int
	operationHost, step, frame, generation                    int
	logs                                                      []string
	vmRunning, maintenance                                    bool
}

func initialModel() model {
	m := model{width: 120, height: 32, page: "dashboard", baseline: "7dcdd87373d72e7a2173e4db02e20d3d7f495cfd", baselineTest: "VM passed · seeded DB", chosen: map[string]bool{}}
	if err := json.Unmarshal(hostData, &m.hosts); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(lineData, &m.lines); err != nil {
		panic(err)
	}
	for i := range m.lines {
		m.lines[i].Tested = "VM passed / unit report"
	}
	for i := range m.hosts {
		h := &m.hosts[i]
		h.Target = "root@" + h.Name + ".example.test"
		h.Revision = fmt.Sprintf("c%06d", i+1)
		h.Running = h.Revision
		h.Server = "ok"
		h.LastDeploy = "today 10:24"
		if i%5 == 1 {
			h.Revision = "d" + h.Revision[1:]
		}
		if i%7 == 2 {
			h.System = "0a3b671"
		}
		if i%9 == 3 {
			h.Server = "unreachable"
		}
		if i%11 == 4 {
			h.Server = "drift"
			h.Running = "9ee031a"
		}
	}
	m.events = []event{{"10:24", "Demo observations loaded; targets and deployment history are fictional."}, {"10:21", "App pins and release lines copied from the builder snapshot."}}
	return m
}
func (m model) Init() tea.Cmd { return nil }
func short(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
func (m model) visible() []int {
	result := []int{}
	for i, h := range m.hosts {
		text := h.Name + " " + h.Product + " " + m.apps(h) + " " + m.system(h) + " " + h.Server + " " + m.repoState(h)
		for _, in := range h.Instances {
			text += " " + in.Name + " " + in.Product + " " + in.Line
		}
		if strings.Contains(strings.ToLower(text), strings.ToLower(m.query)) {
			result = append(result, i)
		}
	}
	return result
}
func (m model) hostIndex() int {
	ids := m.visible()
	if len(ids) == 0 {
		return -1
	}
	return ids[min(m.cursor, len(ids)-1)]
}
func (m model) count() int {
	switch m.tab {
	case 0:
		return len(m.visible())
	case 1:
		return len(m.lines)
	case 4:
		return len(m.events)
	}
	return 1
}
func (m model) capacity() int { return max(1, m.height-10) }
func (m model) bounds(n int) (int, int) {
	size := m.capacity()
	start := 0
	if m.cursor >= size {
		start = m.cursor - size + 1
	}
	if start+size > n {
		start = max(0, n-size)
	}
	return start, min(n, start+size)
}
func (m model) line(name string) release {
	for _, l := range m.lines {
		if l.Name == name {
			return l
		}
	}
	return release{}
}
func (m model) apps(h host) string {
	if len(h.Instances) == 0 {
		return "-"
	}
	diffs := map[string]bool{}
	for _, in := range h.Instances {
		l := m.line(in.Line)
		if l.Name == "" {
			return "unblessed"
		}
		for repo, rev := range in.Repos {
			if l.Repos[repo] != rev {
				diffs[repo] = true
			}
		}
	}
	if len(diffs) == 0 {
		return "blessed"
	}
	names := []string{}
	for r := range diffs {
		names = append(names, r)
	}
	sort.Strings(names)
	return "differs:" + strings.Join(names, "+")
}
func (m model) system(h host) string {
	if h.System == "" {
		return "unpinned"
	}
	if h.System == m.baseline {
		return "baseline"
	}
	return "behind:" + short(h.System)
}
func (m model) repoState(h host) string {
	if h.Running == "" {
		return "never-deployed"
	}
	if h.Revision != h.Running {
		return "pending"
	}
	return "current"
}
func (m *model) record(s string) {
	m.events = append([]event{{time.Now().Format("15:04"), s}}, m.events...)
}
func (m *model) changed(h *host) { h.Revision = fmt.Sprintf("e%06d", len(m.events)+1) }
func (m model) repos() []string {
	id := m.hostIndex()
	if id < 0 || len(m.hosts[id].Instances) == 0 {
		return nil
	}
	in := m.hosts[id].Instances[m.instanceIndex%len(m.hosts[id].Instances)]
	names := []string{}
	for r := range in.Repos {
		names = append(names, r)
	}
	sort.Strings(names)
	return names
}
func (m model) steps() []string {
	switch m.action {
	case "Update apps":
		return []string{"Resolve release pins and product extensions", "Validate app pins; preserve system baseline", "Record mock app-version commit"}
	case "Patch system":
		return []string{"Read tested system baseline", "Replace system pin; preserve every app pin", "Record mock system-patch commit"}
	case "Hold repo":
		return []string{"Resolve repository revision / branch offset", "Replace selected instance repository pin", "Record mock repository hold"}
	case "Bless line":
		return []string{"Resolve branch heads and extensions", "Build seeded test VM", "Check migrated database and service health", "Report unit tests (non-blocking)", "Record blessed pins"}
	case "Baseline":
		return []string{"Resolve nixos-26.05 channel head", "Build seeded allinone VM", "Check system and service health", "Report unit tests (non-blocking)", "Record system baseline; hosts unchanged"}
	case "Build only":
		return []string{"Select committed revision", "Build complete NixOS closure", "Report store path; no activation or tag"}
	case "VM start":
		return []string{"Build allinone VM", "Open persistent mock disk", "Boot services; check /status/server"}
	case "VM reset":
		return []string{"Stop mock VM", "Reset mock persistent disk", "Ready for fresh install"}
	case "Maintenance":
		return []string{"Connect to local mock VM", "Toggle maintenance response (HTTP 503)", "Report HTML and JSON maintenance pages"}
	default:
		return []string{"Check committed, pushed repository state", "Build NixOS system closure", "Transfer closure and activate configuration", "Verify configuration revision and services", "Record deployment tag"}
	}
}
func (m *model) prepare(action string) {
	m.action = action
	m.input = ""
	m.head = false
	m.skipTests = false
	m.buildOnly = false
	m.targets = nil
	if m.tab == 0 {
		for _, id := range m.visible() {
			if m.chosen[m.hosts[id].Name] {
				m.targets = append(m.targets, id)
			}
		}
		if len(m.targets) == 0 {
			if id := m.hostIndex(); id >= 0 {
				m.targets = []int{id}
			}
		}
		if len(m.targets) == 0 {
			m.notice = "No matching hosts."
			return
		}
		if action == "Hold repo" {
			m.targets = []int{m.hostIndex()}
			if len(m.repos()) == 0 {
				m.notice = "DB-only host: no application repositories."
				return
			}
			m.input = "~1"
		}
	}
	m.page = "confirm"
	m.notice = ""
}
func (m *model) finishHost() {
	if len(m.targets) == 0 {
		return
	}
	h := &m.hosts[m.targets[m.operationHost]]
	switch m.action {
	case "Update apps":
		for i := range h.Instances {
			in := &h.Instances[i]
			l := m.line(in.Line)
			for r := range in.Repos {
				if rev, ok := l.Repos[r]; ok {
					in.Repos[r] = rev
					if m.head {
						in.Repos[r] = "f00ba12"
					}
				}
			}
		}
		m.changed(h)
	case "Patch system":
		h.System = m.baseline
		m.changed(h)
	case "Hold repo":
		in := &h.Instances[m.instanceIndex%len(h.Instances)]
		repo := m.repos()[m.repoIndex%len(m.repos())]
		rev := m.input
		if strings.HasPrefix(rev, "~") {
			rev = "a11ce01"
		}
		in.Repos[repo] = rev
		m.changed(h)
	case "Deploy":
		h.Running = h.Revision
		h.Server = "ok"
		h.LastDeploy = "just now"
	}
	m.record(m.action + " · " + h.Name)
}
func (m *model) finishGlobal() {
	switch m.action {
	case "Bless line":
		l := &m.lines[m.cursor]
		for r := range l.Repos {
			l.Repos[r] = "b1e55ed"
		}
		l.Tested = "VM passed / 2 unit failures (reported)"
		if m.skipTests {
			l.Tested = "untested"
		}
	case "Baseline":
		m.baseline = "c0ffee2"
		m.baselineTest = "VM passed / 2 unit failures (reported)"
		if m.skipTests {
			m.baselineTest = "untested"
		}
	case "VM start":
		m.vmRunning = true
	case "VM reset":
		m.vmRunning = false
		m.maintenance = false
	case "Maintenance":
		m.maintenance = !m.maintenance
	}
	m.record(m.action + " · complete")
}
func pulse(g int) tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{g} })
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		if !m.busy || msg.Generation != m.generation {
			return m, nil
		}
		m.frame++
		if m.frame%4 == 0 {
			if m.fail && m.step == 2 {
				m.busy = false
				m.failed = true
				m.logs = append(m.logs, "Mock failure: current target unchanged; completed targets retained.")
				m.record(m.action + " · failed")
				return m, nil
			}
			label := m.steps()[m.step]
			if m.skipTests && (m.action == "Baseline" || m.action == "Bless line") && m.step >= 1 && m.step <= 3 {
				label = "Skipped · " + label
			}
			m.logs = append(m.logs, label)
			if m.step == 3 && !m.skipTests && (m.action == "Baseline" || m.action == "Bless line") {
				m.logs = append(m.logs, "Unit report: 124 passed, 2 failed · non-blocking")
			}
			m.step++
			if m.step == len(m.steps()) {
				if len(m.targets) > 0 {
					m.finishHost()
					m.operationHost++
					if m.operationHost < len(m.targets) {
						m.step = 0
						return m, pulse(m.generation)
					}
				} else {
					m.finishGlobal()
				}
				m.busy = false
				return m, nil
			}
		}
		return m, pulse(m.generation)
	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if m.searching {
			switch k {
			case "esc":
				m.query = ""
				m.searching = false
			case "enter":
				m.searching = false
			case "backspace":
				r := []rune(m.query)
				if len(r) > 0 {
					m.query = string(r[:len(r)-1])
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.query += string(msg.Runes)
				}
			}
			m.cursor = 0
			return m, nil
		}
		if m.busy {
			if k == "esc" {
				m.busy = false
				m.generation++
				m.page = "dashboard"
				m.record(m.action + " · cancelled; completed targets retained")
			}
			if k == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.page == "confirm" {
			switch k {
			case "esc":
				m.page = "dashboard"
			case "enter":
				if m.action == "Hold repo" && !validPin(m.input) {
					m.notice = "Use a 7–40 digit hex commit, or ~N (for example ~1)."
					return m, nil
				}
				if m.action == "Deploy" && m.buildOnly {
					m.action = "Build only"
				}
				m.page = "run"
				m.busy = true
				m.failed = false
				m.step = 0
				m.frame = 0
				m.operationHost = 0
				m.logs = nil
				m.generation++
				return m, pulse(m.generation)
			case "backspace":
				if m.action == "Hold repo" && len(m.input) > 0 {
					m.input = m.input[:len(m.input)-1]
				}
			case "[":
				if m.action == "Hold repo" {
					m.repoIndex = max(0, m.repoIndex-1)
				}
			case "]":
				if m.action == "Hold repo" {
					m.repoIndex = (m.repoIndex + 1) % len(m.repos())
				}
			default:
				if m.action == "Hold repo" {
					if msg.Type == tea.KeyRunes {
						m.input += string(msg.Runes)
					}
				} else {
					if k == "B" && m.action == "Deploy" {
						m.buildOnly = !m.buildOnly
					}
					if k == "H" && m.action == "Update apps" {
						m.head = !m.head
					}
					if k == "T" && (m.action == "Baseline" || m.action == "Bless line") {
						m.skipTests = !m.skipTests
					}
					if k == "q" {
						return m, tea.Quit
					}
				}
			}
			return m, nil
		}
		if k == "q" {
			return m, tea.Quit
		}
		if m.page == "run" {
			if k == "enter" || k == "esc" {
				m.page = "dashboard"
				m.cursor = min(m.cursor, max(0, m.count()-1))
			}
			return m, nil
		}
		if k == "esc" {
			m.page = "dashboard"
			m.query = ""
			m.cursor = 0
			return m, nil
		}
		switch k {
		case "tab", "right":
			m.tab = (m.tab + 1) % 5
			m.cursor = 0
			m.page = "dashboard"
		case "shift+tab", "left":
			m.tab = (m.tab + 4) % 5
			m.cursor = 0
			m.page = "dashboard"
		case "1", "2", "3", "4", "5":
			m.tab = int(k[0] - '1')
			m.cursor = 0
			m.page = "dashboard"
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
			m.instanceIndex = 0
			m.repoIndex = 0
		case "down", "j":
			m.cursor = min(max(0, m.count()-1), m.cursor+1)
			m.instanceIndex = 0
			m.repoIndex = 0
		case "pgdown":
			m.cursor = min(max(0, m.count()-1), m.cursor+m.capacity())
		case "pgup":
			m.cursor = max(0, m.cursor-m.capacity())
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = max(0, m.count()-1)
		case "/":
			if m.tab == 0 {
				m.searching = true
			}
		case " ":
			if m.tab == 0 {
				if id := m.hostIndex(); id >= 0 {
					name := m.hosts[id].Name
					m.chosen[name] = !m.chosen[name]
				}
			}
		case "A":
			if m.tab == 0 {
				for _, id := range m.visible() {
					m.chosen[m.hosts[id].Name] = true
				}
			}
		case "c":
			m.chosen = map[string]bool{}
		case "enter":
			if m.tab == 0 && m.hostIndex() >= 0 {
				m.page = "detail"
			}
		case "i":
			if id := m.hostIndex(); m.tab == 0 && id >= 0 && len(m.hosts[id].Instances) > 0 {
				m.instanceIndex = (m.instanceIndex + 1) % len(m.hosts[id].Instances)
				m.repoIndex = 0
			}
		case "u":
			if m.tab == 0 {
				m.prepare("Update apps")
			}
		case "p":
			if m.tab == 0 {
				m.prepare("Patch system")
			}
		case "d":
			if m.tab == 0 {
				m.prepare("Deploy")
			}
		case "o":
			if m.tab == 0 {
				m.prepare("Hold repo")
			}
		case "b":
			if m.tab == 1 {
				m.prepare("Bless line")
			}
			if m.tab == 2 {
				m.prepare("Baseline")
			}
		case "s":
			if m.tab == 3 {
				m.prepare("VM start")
			}
		case "m":
			if m.tab == 3 {
				if m.vmRunning {
					m.prepare("Maintenance")
				} else {
					m.notice = "Start the mock VM first."
				}
			}
		case "R":
			if m.tab == 3 {
				m.prepare("VM reset")
			}
		case "l":
			if m.tab == 3 {
				m.notice = "Mock journal: init ✓ · nginx ✓ · app ✓ · /status/server 200"
			}
		case "f":
			m.fail = !m.fail
		case "r":
			w, h, g := m.width, m.height, m.generation
			m = initialModel()
			m.width, m.height, m.generation = w, h, g+1
		case "x":
			if len(m.hosts) < 120 {
				original := len(m.hosts)
				for n := original; n < 120; n++ {
					h := cloneHost(m.hosts[n%original])
					h.Name = fmt.Sprintf("%s-%03d", h.Name, n+1)
					h.Target = "root@" + h.Name + ".example.test"
					m.hosts = append(m.hosts, h)
				}
				m.notice = "120-host scale demo enabled. / search, Space select, A select filtered."
			}
		}
	}
	return m, nil
}
func cloneHost(h host) host {
	if h.Instances != nil {
		instances := make([]instance, len(h.Instances))
		copy(instances, h.Instances)
		h.Instances = instances
	}
	for i := range h.Instances {
		r := map[string]string{}
		for k, v := range h.Instances[i].Repos {
			r[k] = v
		}
		h.Instances[i].Repos = r
	}
	return h
}
func validPin(s string) bool {
	if strings.HasPrefix(s, "~") {
		if len(s) < 2 || len(s) > 6 {
			return false
		}
		for _, r := range s[1:] {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	}
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for _, r := range strings.ToLower(s) {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
