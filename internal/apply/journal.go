package apply

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/EdrenaVosh/squad-up/internal/fsx"
)

// JournalEntry is one line of log.jsonl: an operation that changed the config.
// Backup is the id of the copy taken just before it, i.e. the state to return
// to with `squad undo --to`.
type JournalEntry struct {
	Time    time.Time `json:"time"`
	Command string    `json:"command"` // use, restore or undo
	Preset  string    `json:"preset"`  // the preset equipped afterwards
	Backup  string    `json:"backup,omitempty"`
}

// Journal reads the operation history, oldest first.
func (e Engine) Journal() ([]JournalEntry, error) {
	path := e.Paths.LogFile()
	data, existed, err := fsx.ReadFileIfExists(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !existed {
		return nil, nil
	}
	var entries []JournalEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; scanner.Scan(); line++ {
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		var entry JournalEntry
		if err := json.Unmarshal(text, &entry); err != nil {
			return nil, fmt.Errorf("%s line %d is not valid: %w", path, line, err)
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func (e Engine) appendJournal(entry JournalEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode journal entry: %w", err)
	}
	if err := fsx.AppendFile(e.Paths.LogFile(), append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", e.Paths.LogFile(), err)
	}
	return nil
}
