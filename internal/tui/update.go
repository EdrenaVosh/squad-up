package tui

import "fmt"

// Update is a newer release of squad than the one running, announced in the
// top edge of the main screen's frame.
type Update struct {
	Latest, Current string
}

var cOrange = agentColors["orange"]

// blinkFrames is how many frames the notice stays lit, and then dark: about
// half a second each.
const blinkFrames = 6

// label is the notice for the frame: bold orange, blinking while animation is
// on, so that it is seen at a glance. It goes dim rather than away, so that
// the frame's edge has no hole.
func (u *Update) label(c viewCtx) string {
	color := cOrange
	if c.anim && c.frame/blinkFrames%2 == 1 {
		color = cDim
	}
	return fg(color).Bold(true).Render(fmt.Sprintf("⬆ squad %s is out · run squad update", u.Latest))
}
