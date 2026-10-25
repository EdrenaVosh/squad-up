package workspace

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/EdrenaVosh/squad-up/internal/preset"
)

// MaxTitleRunes is the longest name a person may give a preset.
const MaxTitleRunes = 80

// A preset has two names. The *title* is for people: any text, with capitals,
// spaces, symbols and letters of any alphabet. The *id* (a slug such as
// `cube-creator`) is what commands, folders and state use, so it stays small
// and file-safe. A person types the title; the id is made from it.

// CheckTitle reports why text cannot be the title of a preset.
func CheckTitle(text string) error {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return fmt.Errorf("enter a name")
	case utf8.RuneCountInString(text) > MaxTitleRunes:
		return fmt.Errorf("a name has at most %d characters", MaxTitleRunes)
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return fmt.Errorf("a name cannot contain line breaks or control characters")
		}
	}
	return nil
}

// cyrillic is how the letters of Russian are written in the id.
var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i", 'й': "y",
	'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f",
	'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// Slug makes the id of a preset out of a title: lowercase letters and digits
// with single dashes between words, at most 64 characters. A letter that has no
// plain form (a symbol, an emoji) is left out; a title with nothing usable
// gives "preset".
func Slug(title string) string {
	var b strings.Builder
	dash := true // no dash at the start
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
			continue
		}
		if latin, ok := cyrillic[r]; ok { // a soft or hard sign has no letter of its own, and is not a word break
			b.WriteString(latin)
			dash = dash && latin == ""
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	if slug == "" {
		return "preset"
	}
	return slug
}

// FreeID is the id for a new preset with this title: the slug of the title, or
// that with -2, -3… when a preset already has it (or it is reserved).
func (w Workspace) FreeID(title string) string {
	base := Slug(title)
	id := base
	for n := 2; id == "initial" || w.Store().Exists(id); n++ {
		suffix := fmt.Sprintf("-%d", n)
		id = strings.Trim(base[:min(len(base), 64-len(suffix))], "-") + suffix
	}
	return id
}

// SetTitle changes the title of a stored preset.
func (w Workspace) SetTitle(name, title string) error {
	if name == "initial" {
		return ErrProtected
	}
	if err := CheckTitle(title); err != nil {
		return err
	}
	return w.locked(func() error {
		b, err := w.Store().Load(name)
		if err != nil {
			return err
		}
		b.Preset.Title = strings.TrimSpace(title)
		return w.Store().Replace(*b)
	})
}

// IDFor returns the id and the title for what a person typed as the name of a
// preset: text that is already a valid id is used as it is (and is the title
// too), anything else is a title and gets an id made from it.
func (w Workspace) IDFor(text string) (id, title string) {
	text = strings.TrimSpace(text)
	if preset.ValidName(text) && text != "initial" {
		return text, text
	}
	return w.FreeID(text), text
}

// RenameTo gives a preset the name a person typed and returns its new id. Text
// that is already a valid id renames the preset to it, as before. Any other
// text is a title: the preset gets it, and its id is changed only when the
// words changed, so that retyping the same name does not move the preset.
func (w Workspace) RenameTo(old, text string) (string, error) {
	if old == "initial" {
		return "", ErrProtected
	}
	text = strings.TrimSpace(text)
	if err := CheckTitle(text); err != nil {
		return "", err
	}
	if text == "initial" {
		return "", ErrReservedName
	}
	if preset.ValidName(text) {
		if text != old {
			if err := w.Rename(old, text); err != nil {
				return "", err
			}
		}
		return text, nil
	}
	id := old
	if Slug(text) != old {
		id = w.FreeID(text)
		if err := w.Rename(old, id); err != nil {
			return "", err
		}
	}
	return id, w.SetTitle(id, text)
}
