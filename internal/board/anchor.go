package board

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Anchor is where the panel opens from: the monitor and bar edge of the
// click, and the click in monitor coordinates. It is the wire format
// Roundhouse and Switchyard pass between them, so a panel handed over from
// one to another opens from the same icon.
type Anchor struct {
	Monitor  string     `json:"monitor,omitempty"`
	Edge     string     `json:"edge"`
	X        float64    `json:"x"`
	Y        float64    `json:"y"`
	Width    float64    `json:"width"`
	Height   float64    `json:"height"`
	Reserved [4]float64 `json:"reserved"`
	Center   bool       `json:"center"`
}

// hyprSocket is Hyprland's request socket, or "" outside Hyprland.
func hyprSocket() string {
	sig := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if sig == "" {
		return ""
	}
	var candidates []string
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		candidates = append(candidates, filepath.Join(rt, "hypr", sig, ".socket.sock"))
	}
	candidates = append(candidates, filepath.Join("/tmp/hypr", sig, ".socket.sock"))
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// hyprQuery sends one request to Hyprland's socket and returns the reply.
func hyprQuery(sock, cmd string) ([]byte, error) {
	conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(conn, 4<<20))
}

// HyprAnchor asks Hyprland where the cursor and the bar (layer namespace
// ns) are. ok is false outside Hyprland. With center, the anchor only
// names the monitor and the panel opens in its middle.
func HyprAnchor(ns string, center bool) (Anchor, bool) {
	sock := hyprSocket()
	if sock == "" {
		return Anchor{}, false
	}
	monitors, err := hyprQuery(sock, "j/monitors")
	if err != nil {
		return Anchor{}, false
	}
	var cursor, layers []byte
	if !center {
		cursor, _ = hyprQuery(sock, "j/cursorpos")
	}
	layers, _ = hyprQuery(sock, "j/layers")
	a, err := ComputeAnchor(cursor, monitors, layers, ns, center)
	return a, err == nil
}

type hyprMonitor struct {
	Name      string    `json:"name"`
	X         float64   `json:"x"`
	Y         float64   `json:"y"`
	Width     float64   `json:"width"`
	Height    float64   `json:"height"`
	Scale     float64   `json:"scale"`
	Transform int       `json:"transform"`
	Reserved  []float64 `json:"reserved"`
	Focused   bool      `json:"focused"`
}

// logical is the monitor's size in layout pixels.
func (m hyprMonitor) logical() (float64, float64) {
	s := m.Scale
	if s <= 0 {
		s = 1
	}
	w, h := m.Width/s, m.Height/s
	if m.Transform%2 == 1 {
		return h, w
	}
	return w, h
}

func (m hyprMonitor) contains(x, y float64) bool {
	w, h := m.logical()
	return x >= m.X && x < m.X+w && y >= m.Y && y < m.Y+h
}

type hyprLayer struct {
	X, Y, W, H float64
	Namespace  string `json:"namespace"`
}

// ComputeAnchor works the anchor out of raw Hyprland replies: the monitor
// under the cursor, the click in its coordinates, and the bar's edge from
// its layer surface (or, with no bar found, the edge with the most
// reserved space).
func ComputeAnchor(cursorJSON, monitorsJSON, layersJSON []byte, ns string, center bool) (Anchor, error) {
	var monitors []hyprMonitor
	if err := json.Unmarshal(monitorsJSON, &monitors); err != nil {
		return Anchor{}, err
	}
	if len(monitors) == 0 {
		return Anchor{}, errors.New("no monitors")
	}
	var cursor *struct{ X, Y float64 }
	if len(cursorJSON) > 0 {
		var c struct{ X, Y float64 }
		if json.Unmarshal(cursorJSON, &c) == nil {
			cursor = &c
		}
	}
	mon := -1
	if cursor != nil {
		for i, m := range monitors {
			if m.contains(cursor.X, cursor.Y) {
				mon = i
				break
			}
		}
	}
	if mon < 0 {
		for i, m := range monitors {
			if m.Focused {
				mon = i
				break
			}
		}
	}
	if mon < 0 {
		mon = 0
	}
	m := monitors[mon]
	w, h := m.logical()
	var reserved [4]float64
	for i := 0; i < 4 && i < len(m.Reserved); i++ {
		reserved[i] = m.Reserved[i]
	}
	cx, cy := w/2, h/2
	if cursor != nil && !center {
		cx, cy = cursor.X-m.X, cursor.Y-m.Y
	}

	var layers map[string]struct {
		Levels map[string][]hyprLayer `json:"levels"`
	}
	_ = json.Unmarshal(layersJSON, &layers)
	edge := ""
	if ml, ok := layers[m.Name]; ok {
		for _, level := range ml.Levels {
			for _, l := range level {
				if l.Namespace != ns {
					continue
				}
				bx, by := l.X-m.X, l.Y-m.Y
				switch {
				case l.W >= l.H && by+l.H/2 < h/2:
					edge = "top"
				case l.W >= l.H:
					edge = "bottom"
				case bx+l.W/2 < w/2:
					edge = "left"
				default:
					edge = "right"
				}
			}
		}
	}
	if edge == "" {
		edge = "bottom"
		best := 0.0
		for i, side := range []string{"left", "top", "right", "bottom"} {
			if reserved[i] > best {
				best, edge = reserved[i], side
			}
		}
	}
	return Anchor{
		Monitor: m.Name, Edge: edge, X: cx, Y: cy, Width: w, Height: h,
		Reserved: reserved, Center: center,
	}, nil
}
