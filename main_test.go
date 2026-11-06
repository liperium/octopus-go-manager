package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"reflect"
	"strings"
	"testing"
)

func key(m model, s string) model {
	k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	kinds := map[string]tea.KeyType{"enter": tea.KeyEnter, "esc": tea.KeyEsc, "backspace": tea.KeyBackspace, "pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp, "end": tea.KeyEnd, "home": tea.KeyHome}
	if kind, ok := kinds[s]; ok {
		k = tea.KeyMsg{Type: kind}
	}
	next, _ := m.Update(k)
	return next.(model)
}
func complete(t *testing.T, m model) model {
	t.Helper()
	for n := 0; m.busy && n < 4000; n++ {
		next, _ := m.Update(tickMsg{m.generation})
		m = next.(model)
	}
	if m.busy {
		t.Fatal("operation never finished")
	}
	return m
}
func execute(t *testing.T, m model, action string) model {
	t.Helper()
	m.prepare(action)
	if m.page != "confirm" {
		t.Fatalf("no preview: %s", m.notice)
	}
	return complete(t, key(m, "enter"))
}
func appHost(m model) int {
	for i, h := range m.hosts {
		if len(h.Instances) > 0 {
			return i
		}
	}
	return -1
}
func TestSystemAndApplicationIndependence(t *testing.T) {
	m := initialModel()
	id := appHost(m)
	m.cursor = id
	h := &m.hosts[id]
	h.System = "old"
	before := cloneHost(*h)
	m = execute(t, m, "Patch system")
	if !reflect.DeepEqual(before.Instances, m.hosts[id].Instances) || m.hosts[id].System != m.baseline || m.hosts[id].Running != before.Running {
		t.Fatal("patch changed apps or deployed configuration")
	}
	m.hosts[id].System = "older"
	system := m.hosts[id].System
	m = key(m, "enter")
	m = execute(t, m, "Update apps")
	if m.hosts[id].System != system || m.apps(m.hosts[id]) != "blessed" {
		t.Fatal("app update changed system or did not adopt blessed pins")
	}
}
func TestBaselineThenBulkPatch(t *testing.T) {
	m := initialModel()
	before := cloneHost(m.hosts[0])
	m.tab = 2
	m = execute(t, m, "Baseline")
	if !reflect.DeepEqual(before, m.hosts[0]) {
		t.Fatal("baseline automatically changed a host")
	}
	if !strings.Contains(m.baselineTest, "unit failures") {
		t.Fatal("missing nonblocking report")
	}
	m = key(key(m, "enter"), "1")
	m = key(m, "A")
	m = execute(t, m, "Patch system")
	for _, h := range m.hosts {
		if h.System != m.baseline {
			t.Fatal("bulk patch missed host")
		}
	}
}
func TestHoldThenUpdateRestoresBlessedPins(t *testing.T) {
	m := initialModel()
	id := appHost(m)
	m.cursor = id
	before := m.hosts[id].System
	m.prepare("Hold repo")
	m.input = "~2"
	m = complete(t, key(m, "enter"))
	if !strings.Contains(m.apps(m.hosts[id]), "differs") {
		t.Fatal("hold not visible")
	}
	m = key(m, "enter")
	m = execute(t, m, "Update apps")
	if m.apps(m.hosts[id]) != "blessed" || m.hosts[id].System != before {
		t.Fatal("update did not clear hold independently")
	}
}
func TestFilteringAndPagingLargeFleet(t *testing.T) {
	m := key(initialModel(), "x")
	if len(m.hosts) != 120 {
		t.Fatal("missing scale demo")
	}
	m = key(m, "end")
	start, end := m.bounds(len(m.visible()))
	if end != 120 || start == 0 {
		t.Fatal("last page not visible")
	}
	m = key(m, "/")
	m = key(m, "emsa")
	m = key(m, "enter")
	if len(m.visible()) == 0 || len(m.visible()) == 120 || m.cursor != 0 {
		t.Fatal("filter failed")
	}
	m = key(m, "A")
	ids := append([]int(nil), m.visible()...)
	m.prepare("Patch system")
	if !reflect.DeepEqual(ids, m.targets) {
		t.Fatal("bulk scope differs from filtered selection")
	}
}
func TestBuildOnlyDoesNotDeploy(t *testing.T) {
	m := initialModel()
	before := cloneHost(m.hosts[0])
	m.prepare("Deploy")
	m = key(m, "B")
	m = complete(t, key(m, "enter"))
	if !reflect.DeepEqual(before, m.hosts[0]) {
		t.Fatal("build only changed host")
	}
}
func TestFailureCancellationAndStaleTicks(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		m := initialModel()
		before := cloneHost(m.hosts[0])
		m.fail = true
		m.prepare("Patch system")
		m = key(m, "enter")
		oldGeneration := m.generation
		if cancel {
			m = key(m, "esc")
		} else {
			m = complete(t, m)
			if !m.failed {
				t.Fatal("missing simulated failure")
			}
		}
		if !reflect.DeepEqual(before, m.hosts[0]) {
			t.Fatal("failed target changed")
		}
		m.page = "dashboard"
		m.fail = false
		m.prepare("Patch system")
		m = key(m, "enter")
		next, _ := m.Update(tickMsg{oldGeneration})
		m = next.(model)
		if m.frame != 0 {
			t.Fatal("stale timer advanced new operation")
		}
	}
}
func TestQueueKeepsCompletedHostsOnCancel(t *testing.T) {
	m := initialModel()
	m.chosen[m.hosts[0].Name] = true
	m.chosen[m.hosts[1].Name] = true
	m.prepare("Patch system")
	m = key(m, "enter")
	for m.operationHost == 0 {
		next, _ := m.Update(tickMsg{m.generation})
		m = next.(model)
	}
	untouched := cloneHost(m.hosts[1])
	m = key(m, "esc")
	if m.hosts[0].System != m.baseline || !reflect.DeepEqual(untouched, m.hosts[1]) {
		t.Fatal("queue cancellation state incorrect")
	}
}
func TestVMAndMaintenance(t *testing.T) {
	m := initialModel()
	m.tab = 3
	m = key(m, "m")
	if m.page == "confirm" {
		t.Fatal("maintenance should require a running VM")
	}
	m = execute(t, m, "VM start")
	m = key(m, "enter")
	m = execute(t, m, "Maintenance")
	if !m.vmRunning || !m.maintenance {
		t.Fatal("VM state incorrect")
	}
	m = key(m, "enter")
	m = execute(t, m, "VM reset")
	if m.vmRunning || m.maintenance {
		t.Fatal("reset failed")
	}
}
func TestViewsFitAndKeepSelectedRowVisible(t *testing.T) {
	for _, size := range [][2]int{{76, 22}, {100, 24}, {120, 32}, {180, 48}} {
		for _, scale := range []bool{false, true} {
			m := initialModel()
			m.width, m.height = size[0], size[1]
			if scale {
				m = key(m, "x")
			}
			m = key(m, "end")
			for tab := 0; tab < 5; tab++ {
				n := m
				n.tab = tab
				n.cursor = max(0, n.count()-1)
				for _, page := range []string{"dashboard", "detail", "confirm", "run"} {
					if tab != 0 && page == "detail" {
						continue
					}
					n.page = page
					n.action = "Deploy"
					if page == "confirm" {
						n.targets = []int{0}
					}
					if page == "run" {
						n.logs = []string{"first", "second", "third"}
						n.failed = true
					}
					view := n.View()
					if lipgloss.Width(view) > n.width || lipgloss.Height(view) > n.height {
						t.Errorf("%v scale=%v tab=%d page=%s: %dx%d", size, scale, tab, page, lipgloss.Width(view), lipgloss.Height(view))
					}
				}
			}
			if !strings.Contains(m.View(), m.hosts[len(m.hosts)-1].Name[:min(12, len(m.hosts[len(m.hosts)-1].Name))]) {
				t.Fatal("last host missing from viewport")
			}
		}
	}
}
func TestPinValidation(t *testing.T) {
	for _, s := range []string{"~0", "~10", "abcdef1", "ABCDEF123456"} {
		if !validPin(s) {
			t.Errorf("rejected %q", s)
		}
	}
	for _, s := range []string{"", "~", "~-1", "~no", "xyz1234", "abc"} {
		if validPin(s) {
			t.Errorf("accepted %q", s)
		}
	}
}
