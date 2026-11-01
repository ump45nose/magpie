package agent

import (
	"path/filepath"

	"github.com/yetone/magpie/internal/proc"
)

// KiroCLI is Kiro's terminal agent, kiro-cli (kiro.dev/docs/cli). magpie
// wires no model into it, so it isn't one of All's agents and has no row
// on the Agents page: the Library gives it the instructions, MCP servers
// and skills it reads from ~/.kiro (library.apps).
//
// It is found by its program alone, where the installer puts it: on PATH,
// in ~/.local/bin, or inside Kiro CLI.app on macOS (where
// provider.KiroExecutable looks too). Never by ~/.kiro: Kiro Crew keeps
// ~/.kiro/crew there, and the Kiro IDE reads the folder as well.
func KiroCLI() *Agent {
	home, _ := homes()
	a := &Agent{ID: "kiro-cli", Name: "Kiro CLI", Icon: "kiro-color", Bin: "kiro-cli"}
	a.detect = func() bool { return kiroCLIBin(home) != "" }
	return a
}

func kiroCLIBin(home string) string {
	if p := binAt("kiro-cli"); p != "" {
		return p
	}
	for _, p := range append([]string{filepath.Join(home, ".local", "bin", "kiro-cli")}, proc.SystemDirs(
		"/Applications/Kiro CLI.app/Contents/MacOS/kiro-cli", "/usr/local/bin/kiro-cli", "/opt/homebrew/bin/kiro-cli")...) {
		if isFile(p) {
			return p
		}
	}
	return ""
}
