//go:build darwin

package sink

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// quarantineAttribute is the extended attribute Gatekeeper and the system
// "downloaded from the Internet" prompt read.
const quarantineAttribute = "com.apple.quarantine"

// quarantineValue is flags;hex-timestamp;agent;uuid. 0083 is the web-download
// flag set browsers write; the trailing uuid field is optional and left empty.
func quarantineValue(now time.Time) []byte {
	return []byte(fmt.Sprintf("0083;%s;FairDrop;", strconv.FormatInt(now.Unix(), 16)))
}

// markFile writes the quarantine attribute on the open file. A filesystem that
// cannot hold it makes this fail, and the caller turns that into a warning.
func markFile(file *os.File, now time.Time) error {
	return unix.Fsetxattr(int(file.Fd()), quarantineAttribute, quarantineValue(now), 0)
}
