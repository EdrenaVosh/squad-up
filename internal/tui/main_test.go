package tui_test

import (
	"os"
	"testing"
	"time"
)

// The screens show times in the local zone. The golden files must not depend on
// the zone of the machine that records or runs them, so the tests run in UTC.
func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}
