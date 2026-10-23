// Package workspace owns the directory next to a prompt file: its takes
// (every generated candidate, append-only, with a JSON sidecar each), an
// optional character sheet and the discards. The pick (the approved image)
// is moved out of it into images/ beside the prompt file, under the
// prompt's name.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Layout constants.
const (
	TakesDir    = "takes"
	DiscardsDir = "discards"
	SheetStem   = "sheet"
	// ImagesDir sits beside the prompt files and holds their picks.
	ImagesDir = "images"
	// PickInfoFile is the pick sidecar inside the workspace.
	PickInfoFile = "pick.json"
)

// Workspace is `<dir>/<stem>/` for the prompt `<dir>/<stem>.md`.
type Workspace struct {
	Dir  string
	Stem string
}

// For returns the workspace of a prompt file path.
func For(promptPath string) Workspace {
	stem := strings.TrimSuffix(filepath.Base(promptPath), filepath.Ext(promptPath))
	return Workspace{Dir: filepath.Join(filepath.Dir(promptPath), stem), Stem: stem}
}

// Take is the sidecar written next to every generated image. It records
// everything needed to explain, reproduce or continue the generation.
type Take struct {
	Number  int       `json:"take"`
	Created time.Time `json:"created"`
	// Image is the image file name inside takes/ ("001.png"). The picked
	// take's image is in images/ instead; ImagePath finds it either way.
	Image      string `json:"image"`
	PromptFile string `json:"prompt_file"`
	Title      string `json:"title"`
	Kind       string `json:"kind,omitempty"`
	// Purpose distinguishes ordinary takes from sheet takes.
	Purpose string `json:"purpose,omitempty"`

	Model       string   `json:"model"`
	AspectRatio string   `json:"aspect_ratio"`
	Resolution  string   `json:"resolution"`
	Temperature *float64 `json:"temperature,omitempty"`
	Seed        *int64   `json:"seed,omitempty"`
	Search      bool     `json:"search,omitempty"`

	// System is the system instruction sent (the style directive).
	System string `json:"system,omitempty"`
	// Text is the full user text sent, preamble included.
	Text string `json:"text"`
	// Images are the reference images sent, in order.
	Images []ImageRef `json:"images,omitempty"`

	// Parent is the take this one refined, 0 for a fresh generation;
	// Instruction is the edit that was asked for.
	Parent      int    `json:"parent,omitempty"`
	Instruction string `json:"instruction,omitempty"`

	Response Response `json:"response"`
	Usage    Usage    `json:"usage"`
}

// ImageRef records a reference image that was sent.
type ImageRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	// Role says why it was sent: "cast:<title>", "sheet:<title>",
	// "reference", "style", "parent".
	Role string `json:"role"`
	// Label is how the preamble named it ("Image 2").
	Label string `json:"label,omitempty"`
}

// Response is the model turn, block by block, so a refinement can replay
// it exactly (text, thought signatures and the image in their positions).
type Response struct {
	Blocks       []Block `json:"blocks"`
	FinishReason string  `json:"finish_reason"`
	ResponseID   string  `json:"response_id,omitempty"`
	ModelVersion string  `json:"model_version,omitempty"`
}

// Block is one part of the model turn.
type Block struct {
	Type string `json:"type"` // text | signature | image
	Text string `json:"text,omitempty"`
	// Signature is an opaque thought signature.
	Signature string `json:"signature,omitempty"`
	// Image is a file name inside takes/ for an image block.
	Image string `json:"image,omitempty"`
}

// Usage is what the call cost.
type Usage struct {
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// Pick is the sidecar of the approved image.
type Pick struct {
	Take     int       `json:"take"`
	PickedAt time.Time `json:"picked_at"`
	SHA256   string    `json:"sha256"`
	// Image is the pick's file name inside images/ ("kaelen.jpg").
	Image string `json:"image"`
	// Source is the take's file name inside takes/ ("002.jpg"), where the
	// image goes back when another take is picked.
	Source string `json:"source"`
}

// Text returns the text blocks of the response joined.
func (r Response) Text() string {
	var parts []string
	for _, b := range r.Blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (w Workspace) takesDir() string    { return filepath.Join(w.Dir, TakesDir) }
func (w Workspace) discardsDir() string { return filepath.Join(w.Dir, DiscardsDir) }

// ImagesDir is images/ beside the prompt file, shared by every prompt in
// that directory.
func (w Workspace) ImagesDir() string { return filepath.Join(filepath.Dir(w.Dir), ImagesDir) }

// TakePath returns the path of a file inside takes/.
func (w Workspace) TakePath(name string) string { return filepath.Join(w.takesDir(), name) }

// ImagePath returns where a take's image file is: takes/, or images/ when
// it is the take that was picked (and so moved).
func (w Workspace) ImagePath(name string) string {
	path := w.TakePath(name)
	if exists(path) {
		return path
	}
	if p, err := w.ReadPick(); err == nil && p.Source == name && exists(w.PickPath()) {
		return w.PickPath()
	}
	return path
}

// imageExts are the extensions a take, and so a pick, can have.
var imageExts = []string{".png", ".jpg", ".webp", ".gif"}

// PickPath is the stable path of the approved image, images/<stem><ext>,
// with the extension of the take that was picked. It does not depend on
// the sidecar; with no pick it is images/<stem>.png.
func (w Workspace) PickPath() string {
	for _, ext := range imageExts {
		if path := filepath.Join(w.ImagesDir(), w.Stem+ext); exists(path) {
			return path
		}
	}
	return filepath.Join(w.ImagesDir(), w.Stem+".png")
}

// PickInfoPath is the pick sidecar.
func (w Workspace) PickInfoPath() string { return filepath.Join(w.Dir, PickInfoFile) }

// SheetPath is the stable path of the character sheet.
func (w Workspace) SheetPath() string { return filepath.Join(w.Dir, SheetStem+".png") }

// HasPick reports whether an approved image exists.
func (w Workspace) HasPick() bool { return exists(w.PickPath()) }

// HasSheet reports whether a sheet exists.
func (w Workspace) HasSheet() bool { return exists(w.SheetPath()) }

// ReadPick returns the pick sidecar.
func (w Workspace) ReadPick() (Pick, error) {
	var p Pick
	data, err := os.ReadFile(w.PickInfoPath())
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(data, &p)
}

// Takes lists every take, ascending.
func (w Workspace) Takes() ([]Take, error) {
	entries, err := os.ReadDir(w.takesDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Take
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		t, err := w.readTake(filepath.Join(w.takesDir(), e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func (w Workspace) readTake(path string) (Take, error) {
	var t Take
	data, err := os.ReadFile(path)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("%s: %w", path, err)
	}
	return t, nil
}

// Take returns one take by number.
func (w Workspace) Take(n int) (Take, error) {
	t, err := w.readTake(w.TakePath(fmt.Sprintf("%03d.json", n)))
	if errors.Is(err, os.ErrNotExist) {
		return t, fmt.Errorf("take %d does not exist in %s", n, w.Dir)
	}
	return t, err
}

// Latest returns the highest-numbered take.
func (w Workspace) Latest() (Take, bool, error) {
	ts, err := w.Takes()
	if err != nil || len(ts) == 0 {
		return Take{}, false, err
	}
	return ts[len(ts)-1], true, nil
}

// Reserve returns the first of n consecutive take numbers that are free,
// creating takes/. Numbers never go backwards, even after a prune: the
// discards keep their numbers and are counted.
func (w Workspace) Reserve(n int) (int, error) {
	if err := os.MkdirAll(w.takesDir(), 0o755); err != nil {
		return 0, err
	}
	max := 0
	for _, dir := range []string{w.takesDir(), w.discardsDir()} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if v, err := strconv.Atoi(stem); err == nil && v > max {
				max = v
			}
		}
	}
	return max + 1, nil
}

// Save writes a take's image and sidecar. The image file name is derived
// from the number and the MIME type; the Image fields are filled in.
func (w Workspace) Save(t Take, image []byte, mime string) (Take, error) {
	if err := os.MkdirAll(w.takesDir(), 0o755); err != nil {
		return t, err
	}
	t.Image = fmt.Sprintf("%03d%s", t.Number, extFor(mime))
	for i, b := range t.Response.Blocks {
		if b.Type == "image" && b.Image == "" {
			t.Response.Blocks[i].Image = t.Image
		}
	}
	imgPath := w.TakePath(t.Image)
	if exists(imgPath) {
		return t, fmt.Errorf("%s already exists; takes are never overwritten", imgPath)
	}
	if err := os.WriteFile(imgPath, image, 0o644); err != nil {
		return t, err
	}
	if err := writeJSON(w.TakePath(fmt.Sprintf("%03d.json", t.Number)), t); err != nil {
		return t, err
	}
	return t, nil
}

// PickTake approves a take: its image is MOVED from takes/ to
// images/<stem><ext> and the pick sidecar records which take it was. A
// previous pick is moved back to takes/ first, so every take keeps its
// image and picking again loses nothing. The take's sidecar stays in
// takes/.
func (w Workspace) PickTake(n int) (Pick, error) {
	t, err := w.Take(n)
	if err != nil {
		return Pick{}, err
	}
	cur, curErr := w.ReadPick()
	if curErr == nil && cur.Take == n && w.HasPick() && !exists(w.TakePath(t.Image)) {
		return cur, nil // already the pick
	}
	src := w.TakePath(t.Image)
	if !exists(src) {
		return Pick{}, fmt.Errorf("take %d has no image at %s", n, src)
	}
	old := w.PickPath()
	var back string
	if exists(old) {
		if curErr != nil || cur.Source == "" {
			return Pick{}, fmt.Errorf("%s exists but %s does not say which take it is; move it away and pick again", old, w.PickInfoPath())
		}
		back = w.TakePath(cur.Source)
		if exists(back) {
			return Pick{}, fmt.Errorf("cannot return the previous pick to %s: the file exists", back)
		}
		if err := os.Rename(old, back); err != nil {
			return Pick{}, err
		}
	}
	if err := os.MkdirAll(w.ImagesDir(), 0o755); err != nil {
		return Pick{}, err
	}
	dst := filepath.Join(w.ImagesDir(), w.Stem+filepath.Ext(t.Image))
	if err := os.Rename(src, dst); err != nil {
		if back != "" {
			_ = os.Rename(back, old) // restore the previous pick
		}
		return Pick{}, err
	}
	sum, err := FileSHA256(dst)
	if err != nil {
		return Pick{}, err
	}
	p := Pick{Take: n, PickedAt: time.Now().UTC(), SHA256: sum, Image: filepath.Base(dst), Source: t.Image}
	return p, writeJSON(w.PickInfoPath(), p)
}

// SaveSheet stores a generated character sheet at the stable sheet path
// (the take is also saved under takes/ by the caller).
func (w Workspace) SaveSheet(image []byte) error {
	return os.WriteFile(w.SheetPath(), image, 0o644)
}

// Prune moves every take that is not the current pick (and not a sheet
// take) to discards/, or deletes them when remove is true. It returns the
// take numbers affected.
func (w Workspace) Prune(remove bool) ([]int, error) {
	ts, err := w.Takes()
	if err != nil {
		return nil, err
	}
	// The pick stays, and so does every take it was refined from: a later
	// refinement replays that chain, and a pruned ancestor would end the
	// thread the pick belongs to.
	keep := map[int]bool{}
	if p, err := w.ReadPick(); err == nil {
		for n := p.Take; n != 0 && !keep[n]; {
			keep[n] = true
			t, err := w.Take(n)
			if err != nil {
				break
			}
			n = t.Parent
		}
	}
	var done []int
	for _, t := range ts {
		if keep[t.Number] || t.Purpose == "sheet" {
			continue
		}
		files := []string{w.TakePath(t.Image), w.TakePath(fmt.Sprintf("%03d.json", t.Number))}
		for _, f := range files {
			if remove {
				if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
					return done, err
				}
				continue
			}
			if err := os.MkdirAll(w.discardsDir(), 0o755); err != nil {
				return done, err
			}
			if err := os.Rename(f, filepath.Join(w.discardsDir(), filepath.Base(f))); err != nil {
				return done, err
			}
		}
		done = append(done, t.Number)
	}
	return done, nil
}

// SHA256 hex-digests data.
func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// FileSHA256 digests a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extFor(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
