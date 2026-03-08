package initramfs

import (
	"fmt"
	"io"
	"io/fs"
	"time"
)

const (
	newcMagic = "070701"
	trailer   = "TRAILER!!!"
)

type Writer struct {
	w       io.Writer
	nextIno uint32
	closed  bool
}

type entry struct {
	Name      string
	Mode      uint32
	UID       uint32
	GID       uint32
	NLink     uint32
	MTime     uint32
	FileSize  uint32
	DevMajor  uint32
	DevMinor  uint32
	RDevMajor uint32
	RDevMinor uint32
	Data      []byte
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, nextIno: 1}
}

func (w *Writer) AddDirectory(name string, perm fs.FileMode, modTime time.Time) error {
	return w.add(entry{
		Name:  name,
		Mode:  uint32(perm.Perm()) | 0o040000,
		NLink: 2,
		MTime: uint32(modTime.Unix()),
	})
}

func (w *Writer) AddFile(name string, perm fs.FileMode, modTime time.Time, data []byte) error {
	return w.add(entry{
		Name:     name,
		Mode:     uint32(perm.Perm()) | 0o100000,
		NLink:    1,
		MTime:    uint32(modTime.Unix()),
		FileSize: uint32(len(data)),
		Data:     data,
	})
}

func (w *Writer) AddCharDevice(name string, perm fs.FileMode, modTime time.Time, major uint32, minor uint32) error {
	return w.add(entry{
		Name:      name,
		Mode:      uint32(perm.Perm()) | 0o020000,
		NLink:     1,
		MTime:     uint32(modTime.Unix()),
		RDevMajor: major,
		RDevMinor: minor,
	})
}

func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	return w.add(entry{
		Name:  trailer,
		Mode:  0,
		NLink: 1,
	})
}

func (w *Writer) add(e entry) error {
	if w.closed && e.Name != trailer {
		return fmt.Errorf("writer closed")
	}
	if e.Name == "" {
		return fmt.Errorf("entry name must not be empty")
	}

	if err := writeString(w.w, fmt.Sprintf(
		"%s%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x%08x",
		newcMagic,
		w.nextIno,
		e.Mode,
		e.UID,
		e.GID,
		e.NLink,
		e.MTime,
		e.FileSize,
		e.DevMajor,
		e.DevMinor,
		e.RDevMajor,
		e.RDevMinor,
		len(e.Name)+1,
		0,
	)); err != nil {
		return err
	}

	if err := writeString(w.w, e.Name); err != nil {
		return err
	}
	if err := writeBytes(w.w, []byte{0}); err != nil {
		return err
	}
	if err := pad4(w.w, 110+len(e.Name)+1); err != nil {
		return err
	}
	if err := writeBytes(w.w, e.Data); err != nil {
		return err
	}
	if err := pad4(w.w, len(e.Data)); err != nil {
		return err
	}

	w.nextIno++
	return nil
}

func writeString(w io.Writer, value string) error {
	_, err := io.WriteString(w, value)
	return err
}

func writeBytes(w io.Writer, value []byte) error {
	if len(value) == 0 {
		return nil
	}

	_, err := w.Write(value)
	return err
}

func pad4(w io.Writer, size int) error {
	padding := (4 - (size % 4)) % 4
	if padding == 0 {
		return nil
	}

	_, err := w.Write(make([]byte, padding))
	return err
}
