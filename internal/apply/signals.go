package apply

import (
	"os"
	"os/signal"
	"syscall"
)

// holdSignals keeps Ctrl-C, SIGTERM and SIGHUP from ending the process while
// files are being written: a half-applied change is worse than a late exit.
// The returned function stops holding and gives back the first signal that
// arrived in the meantime, or nil.
func holdSignals() func() os.Signal {
	ch := make(chan os.Signal, 4)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	return func() os.Signal {
		signal.Stop(ch)
		select {
		case s := <-ch:
			return s
		default:
			return nil
		}
	}
}

// raiseAgain sends a signal that was held back to this process once the work
// is done, so that Ctrl-C still ends the program, only a moment later.
func raiseAgain(s os.Signal) {
	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(s)
	}
}
