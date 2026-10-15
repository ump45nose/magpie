package gui

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

// snWatcher is a StatusNotifierWatcher that tracks registered items by
// service name and drops them when that name's owner goes away — the SNI
// behaviour that made dropTrayName empty the tray on noctalia (#2421).
type snWatcher struct {
	mu    sync.Mutex
	items map[string]bool // service name → registered
}

func (w *snWatcher) RegisterStatusNotifierItem(service string) *dbus.Error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.items == nil {
		w.items = map[string]bool{}
	}
	// Unique-name form (:1.x) or well-known name: record as given. Path
	// form is not used by reregisterTrayByUnique.
	w.items[service] = true
	return nil
}

func (w *snWatcher) list() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.items))
	for k := range w.items {
		out = append(out, k)
	}
	return out
}

func (w *snWatcher) dropOwner(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.items, name)
}

func (w *snWatcher) has(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.items[name]
}

// registerSNI is RegisterStatusNotifierItem as the tray calls it, taking the
// service or path argument directly so tests can drive both forms.
func (w *snWatcher) registerSNI(service string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.items == nil {
		w.items = map[string]bool{}
	}
	w.items[service] = true
}

func startTestBus(t *testing.T) *dbus.Conn {
	t.Helper()
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon is required for tray bus tests")
	}
	cmd := exec.Command(bin, "--session", "--nofork", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	sc := bufio.NewScanner(stdout)
	if !sc.Scan() {
		t.Fatalf("no bus address: %v", sc.Err())
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", sc.Text())
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func exportWatcher(t *testing.T, conn *dbus.Conn) *snWatcher {
	t.Helper()
	w := &snWatcher{items: map[string]bool{}}
	// Export a method that takes the service string. godbus Export uses the
	// method name; the SNI interface name is set via ExportWithMap-style
	// path by exporting under the well-known watcher destination.
	if reply, err := conn.RequestName("org.kde.StatusNotifierWatcher", dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim watcher: %v %v", reply, err)
	}
	intro := `
<node>
  <interface name="org.kde.StatusNotifierWatcher">
    <method name="RegisterStatusNotifierItem">
      <arg direction="in" type="s" name="service"/>
    </method>
  </interface>
` + introspect.IntrospectDataString + `</node>`
	if err := conn.Export(w, "/StatusNotifierWatcher", "org.kde.StatusNotifierWatcher"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Export(introspect.Introspectable(intro), "/StatusNotifierWatcher", "org.freedesktop.DBus.Introspectable"); err != nil {
		t.Fatal(err)
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	); err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig == nil || sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) < 3 {
				continue
			}
			name, _ := sig.Body[0].(string)
			newOwner, _ := sig.Body[2].(string)
			if newOwner == "" && strings.HasPrefix(name, "org.kde.StatusNotifierItem-") {
				w.dropOwner(name)
			}
		}
	}()
	return w
}

// Releasing the well-known tray name makes an SNI-spec watcher drop the
// item; without a re-register the bar has nothing left (#2421).
func TestReleaseTrayNameUnregistersWithoutReregister(t *testing.T) {
	bus := startTestBus(t)
	w := exportWatcher(t, bus)

	tray, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tray.Close() })
	wellKnown := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", 42421)
	if reply, err := tray.RequestName(wellKnown, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim tray name: %v %v", reply, err)
	}
	w.registerSNI(wellKnown)
	if !w.has(wellKnown) {
		t.Fatal("item not registered")
	}
	if _, err := tray.ReleaseName(wellKnown); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !w.has(wellKnown) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if w.has(wellKnown) {
		t.Fatalf("item still registered after ReleaseName: %v", w.list())
	}
}

// dropTrayNameOn re-registers by the connection's unique name so the
// watcher still has an item after the well-known name is released (#2421).
func TestDropTrayNameReregistersByUnique(t *testing.T) {
	bus := startTestBus(t)
	w := exportWatcher(t, bus)

	tray, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tray.Close() })
	wellKnown := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", 42422)
	if reply, err := tray.RequestName(wellKnown, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("claim tray name: %v %v", reply, err)
	}
	w.registerSNI(wellKnown)
	unique := tray.Names()[0]

	// Wire RegisterStatusNotifierItem on the exported watcher so the
	// re-register call from dropTrayNameOn is recorded.
	// (Export already points at w; RegisterStatusNotifierItem will run.)

	dropTrayNameOn(tray, wellKnown)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.has(unique) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if w.has(wellKnown) {
		t.Fatalf("well-known name still registered (Omarchy duplicate): %v", w.list())
	}
	if !w.has(unique) {
		t.Fatalf("unique name not registered after drop: have %v, want %s", w.list(), unique)
	}
}
