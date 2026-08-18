package detect

import (
	"os/exec"
	"sort"
	"strings"
)

type PlayerInfo struct {
	Name     string   `json:"name"`
	Binary   string   `json:"binary"`
	Priority int      `json:"priority"`
	Args     []string `json:"args,omitempty"`
}

var knownPlayers = []PlayerInfo{
	{Name: "mpv", Binary: "mpv", Priority: 10, Args: []string{"--hwdec=auto"}},
	{Name: "vlc", Binary: "vlc", Priority: 20, Args: []string{"--play-and-exit"}},
	{Name: "celluloid", Binary: "celluloid", Priority: 30, Args: nil},
	{Name: "iina", Binary: "iina", Priority: 40, Args: nil},
	{Name: "mpc-hc", Binary: "mpc-hc", Priority: 50, Args: nil},
	{Name: "potplayer", Binary: "PotPlayer", Priority: 60, Args: nil},
}

func Available() []PlayerInfo {
	var found []PlayerInfo
	for _, p := range knownPlayers {
		if _, err := exec.LookPath(p.Binary); err == nil {
			found = append(found, p)
		}
	}
	return found
}

func Best() *PlayerInfo {
	players := Available()
	if len(players) == 0 {
		if _, err := exec.LookPath("mpv"); err == nil {
			return &PlayerInfo{Name: "mpv", Binary: "mpv", Priority: 10}
		}
		if _, err := exec.LookPath("vlc"); err == nil {
			return &PlayerInfo{Name: "vlc", Binary: "vlc", Priority: 20}
		}
		return nil
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].Priority < players[j].Priority
	})
	return &players[0]
}

func ByPriority(priority []string) []PlayerInfo {
	avail := Available()
	availMap := make(map[string]PlayerInfo)
	for _, p := range avail {
		availMap[strings.ToLower(p.Name)] = p
	}

	var ordered []PlayerInfo
	seen := make(map[string]bool)

	for _, name := range priority {
		name = strings.ToLower(name)
		if p, ok := availMap[name]; ok && !seen[name] {
			ordered = append(ordered, p)
			seen[name] = true
		}
	}

	for _, p := range avail {
		if !seen[strings.ToLower(p.Name)] {
			ordered = append(ordered, p)
			seen[strings.ToLower(p.Name)] = true
		}
	}

	return ordered
}
