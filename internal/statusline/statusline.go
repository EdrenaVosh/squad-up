// Package statusline puts the equipped preset into Claude Code's status line.
// `enable` replaces the `statusLine` of settings.json with a command of squad
// that runs the user's own line first and appends the preset; `disable`
// puts the original back.
package statusline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/EdrenaVosh/squad-up/internal/apply"
	"github.com/EdrenaVosh/squad-up/internal/fsx"
	"github.com/EdrenaVosh/squad-up/internal/paths"
	"github.com/EdrenaVosh/squad-up/internal/settings"
)

// ErrNotEnabled means `squad statusline enable` was never run (or was undone).
var ErrNotEnabled = errors.New("the squad status line is not enabled")

// Outcome says what Enable or Disable did.
type Outcome int

// Outcomes.
const (
	Enabled        Outcome = iota + 1 // our line was installed
	AlreadyEnabled                    // it was there already, unchanged
	Refreshed                         // it was there; only the command changed
	Restored                          // the original line is back
	NotOurs                           // the line was changed by someone else; left alone
)

// Record is what squad keeps about the line it replaced.
type Record struct {
	Schema    int             `json:"schema"`
	EnabledAt time.Time       `json:"enabledAt"`
	Installed json.RawMessage `json:"installed"`          // the statusLine squad wrote
	Original  json.RawMessage `json:"original,omitempty"` // the statusLine it replaced; empty when there was none
}

// Load reads the record, or returns nil when there is none.
func Load(p paths.Paths) (*Record, error) {
	data, existed, err := fsx.ReadFileIfExists(p.StatuslineFile())
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p.StatuslineFile(), err)
	}
	if !existed {
		return nil, nil
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("%s is not valid: %w", p.StatuslineFile(), err)
	}
	return &rec, nil
}

func save(p paths.Paths, rec Record) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return fsx.WriteFileAtomic(p.StatuslineFile(), append(data, '\n'), 0o600)
}

// readSettings returns settings.json and whether it exists, and refuses a file
// that is not a JSON object.
func readSettings(p paths.Paths) ([]byte, bool, error) {
	doc, existed, err := fsx.ReadFileIfExists(p.SettingsFile())
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", p.SettingsFile(), err)
	}
	if existed && (!gjson.ValidBytes(doc) || !gjson.ParseBytes(doc).IsObject()) {
		return nil, false, fmt.Errorf("%s is not a valid JSON object: fix it first, squad will not rewrite it", p.SettingsFile())
	}
	return doc, existed, nil
}

func writeSettings(p paths.Paths, doc []byte) error {
	mode := os.FileMode(0o600)
	if info, err := os.Stat(p.SettingsFile()); err == nil {
		mode = info.Mode().Perm()
	}
	return fsx.WriteFileAtomic(p.SettingsFile(), doc, mode)
}

// Enable installs command as the status line. The user's own line is kept in
// the record and run by the command. Calling it again changes nothing, except
// that a new command (the binary moved) replaces the old one.
func Enable(p paths.Paths, command string, now time.Time) (Outcome, error) {
	unlock, err := fsx.Lock(p.LockFile())
	if err != nil {
		return 0, err
	}
	defer unlock()

	doc, existed, err := readSettings(p)
	if err != nil {
		return 0, err
	}
	rec, err := Load(p)
	if err != nil {
		return 0, err
	}

	current := gjson.GetBytes(doc, "statusLine")
	var original json.RawMessage
	outcome := Enabled
	if rec != nil && current.Exists() && current.Get("command").String() == gjson.GetBytes(rec.Installed, "command").String() {
		if current.Get("command").String() == command {
			return AlreadyEnabled, nil
		}
		original, outcome = rec.Original, Refreshed
	} else if current.Exists() {
		original = json.RawMessage(current.Raw)
	}

	base := []byte(`{}`)
	if len(original) > 0 {
		base = bytes.Clone(original)
	}
	installed, err := sjson.SetBytes(base, "type", "command")
	if err == nil {
		installed, err = sjson.SetBytes(installed, "command", command)
	}
	if err != nil {
		return 0, fmt.Errorf("build the status line: %w", err)
	}
	if len(original) == 0 {
		installed = compact(installed)
	}

	edited, err := settings.Apply(doc, []settings.Edit{{Key: "statusLine", Raw: string(installed)}})
	if err != nil {
		return 0, fmt.Errorf("edit %s: %w", p.SettingsFile(), err)
	}
	// The backup is of the settings as they were before squad touched them;
	// a refresh finds squad's own line in them and must not replace it.
	if _, err := os.Stat(p.StatuslineBackupFile()); existed && (outcome != Refreshed || err != nil) {
		if err := fsx.WriteFileAtomic(p.StatuslineBackupFile(), doc, 0o600); err != nil {
			return 0, fmt.Errorf("back up settings.json: %w", err)
		}
	}
	if err := writeSettings(p, edited); err != nil {
		return 0, fmt.Errorf("write %s: %w", p.SettingsFile(), err)
	}
	record := Record{Schema: 1, EnabledAt: now.UTC(), Installed: installed, Original: original}
	if err := save(p, record); err != nil {
		return 0, fmt.Errorf("save the record in %s: %w (run `squad statusline disable` to undo)", p.StatuslineFile(), err)
	}
	return outcome, nil
}

func compact(raw []byte) []byte {
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return raw
	}
	return buf.Bytes()
}

// Disable puts the user's original line back, or removes ours when there was
// none. A line that someone changed after Enable is left alone.
func Disable(p paths.Paths) (Outcome, error) {
	unlock, err := fsx.Lock(p.LockFile())
	if err != nil {
		return 0, err
	}
	defer unlock()

	rec, err := Load(p)
	if err != nil {
		return 0, err
	}
	if rec == nil {
		return 0, ErrNotEnabled
	}
	doc, _, err := readSettings(p)
	if err != nil {
		return 0, err
	}

	outcome := NotOurs
	current := gjson.GetBytes(doc, "statusLine")
	if current.Exists() && current.Get("command").String() == gjson.GetBytes(rec.Installed, "command").String() {
		edit := settings.Edit{Key: "statusLine", Delete: len(rec.Original) == 0, Raw: string(rec.Original)}
		edited, err := settings.Apply(doc, []settings.Edit{edit})
		if err != nil {
			return 0, fmt.Errorf("edit %s: %w", p.SettingsFile(), err)
		}
		if err := writeSettings(p, edited); err != nil {
			return 0, fmt.Errorf("write %s: %w", p.SettingsFile(), err)
		}
		outcome = Restored
	}
	if err := fsx.RemoveAll(p.StatuslineFile()); err != nil {
		return 0, err
	}
	return outcome, nil
}

// Input is what the status line command is given.
type Input struct {
	Stdin     []byte // the JSON Claude Code sends, passed on to the user's line
	RunPreset string // the preset of `squad run`, from the environment
	Color     bool
}

// Render builds the status line: the output of the user's own line, if
// there was one, with the preset appended to its last line. It never fails: a
// broken or slow user line only costs its own output.
func Render(ctx context.Context, p paths.Paths, in Input) string {
	segment := segmentOf(p, in)

	var base string
	if rec, err := Load(p); err == nil && rec != nil {
		if command := gjson.GetBytes(rec.Original, "command").String(); command != "" {
			base = strings.TrimRight(runShell(ctx, command, in.Stdin), "\r\n")
		}
	}
	switch {
	case segment == "":
		return base
	case base == "":
		return segment
	}
	return base + " " + segment
}

func segmentOf(p paths.Paths, in Input) string {
	text := ""
	switch {
	case in.RunPreset != "":
		text = "◆ " + in.RunPreset + " (run)"
	default:
		state, err := apply.LoadState(p)
		if err != nil || state.Active == "" || state.Active == "initial" {
			return ""
		}
		text = "◆ " + state.Active
	}
	if in.Color {
		return "\x1b[38;5;178m" + text + "\x1b[0m"
	}
	return text
}

// runShell runs a shell command with stdin and returns what it printed.
func runShell(ctx context.Context, command string, stdin []byte) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout = &out
	// A command killed on timeout may leave a child that still holds the pipe.
	cmd.WaitDelay = 500 * time.Millisecond
	_ = cmd.Run() // a failing line still shows what it printed
	return out.String()
}
