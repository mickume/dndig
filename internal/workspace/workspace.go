// Package workspace owns the directory next to a prompt file: its takes
// (every generated candidate, append-only, with a JSON sidecar each), the
// pick (the approved image, at a stable name), an optional character sheet,
// and the discards.
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
	// Image is the image file name inside takes/ ("001.png").
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
	Image    string    `json:"image"`
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

// TakePath returns the path of a file inside takes/.
func (w Workspace) TakePath(name string) string { return filepath.Join(w.takesDir(), name) }

// PickPath is the stable path of the approved image.
func (w Workspace) PickPath() string { return filepath.Join(w.Dir, w.Stem+".png") }

// PickInfoPath is the pick sidecar.
func (w Workspace) PickInfoPath() string { return filepath.Join(w.Dir, w.Stem+".json") }

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

// PickTake approves a take: the image is COPIED to the stable pick path and
// the pick sidecar records which take it was. A previous pick is replaced;
// its take is still in takes/.
func (w Workspace) PickTake(n int) (Pick, error) {
	t, err := w.Take(n)
	if err != nil {
		return Pick{}, err
	}
	src := w.TakePath(t.Image)
	data, err := os.ReadFile(src)
	if err != nil {
		return Pick{}, err
	}
	if err := os.WriteFile(w.PickPath(), data, 0o644); err != nil {
		return Pick{}, err
	}
	p := Pick{Take: n, PickedAt: time.Now().UTC(), SHA256: SHA256(data), Image: filepath.Base(w.PickPath())}
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
	keep := map[int]bool{}
	if p, err := w.ReadPick(); err == nil {
		keep[p.Take] = true
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
