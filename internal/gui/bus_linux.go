package gui

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// sessionBus reports whether a D-Bus session bus is reachable; Wails'
// single-instance lock needs one and gives up on the whole app without it.
func sessionBus() bool {
	_, err := dbus.SessionBus()
	return err == nil
}

// dropTrayName gives up the org.kde.StatusNotifierItem-<pid>-1 name Wails'
// tray takes. The tray registers with the watcher by its path, which the
// watcher files under the connection's own name (:1.x); anything that also
// registers the well-known name (songlairui on X: 为什么我的 omarchy 上会有
// 两只喜鹊) makes the same icon a second item, as Quickshell's watcher tells
// items apart by how they were named, not by who owns them. Without the name
// such a registration is refused, and one made already is dropped with it.
//
// After the release, the item is registered again by the connection's unique
// name (#2421): watchers that follow the SNI spec and track the service
// name's owner (noctalia and similar) unregister the item when the
// well-known name is released; a RegisterStatusNotifierItem of the unique
// name puts it back without bringing the Omarchy duplicate.
func dropTrayName() {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	// the tray takes it when the app has started, a moment after this
	for range 120 {
		var owner string
		if conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, name).Store(&owner) == nil {
			dropTrayNameOn(conn, name)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// dropTrayNameOn releases the well-known tray name on conn and registers
// the StatusNotifierItem again by conn's unique name (#2421).
func dropTrayNameOn(conn *dbus.Conn, name string) {
	if _, err := conn.ReleaseName(name); err != nil {
		log.Println("tray: release", name+":", err)
		return
	}
	reregisterTrayByUnique(conn)
}

// reregisterTrayByUnique asks the StatusNotifierWatcher to keep the item
// under this connection's unique name, so releasing the well-known name
// does not leave the bar empty (#2421).
func reregisterTrayByUnique(conn *dbus.Conn) {
	unique := conn.Names()
	if len(unique) == 0 {
		return
	}
	id := unique[0]
	if !strings.HasPrefix(id, ":") {
		return
	}
	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
	if call := obj.Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, id); call.Err != nil {
		log.Println("tray: re-register by unique name:", call.Err)
	}
}
