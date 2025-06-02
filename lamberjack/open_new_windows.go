package lamberjack

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

func (l *Logger) openNew() error {
	err := os.MkdirAll(l.dir(), 0755)
	if err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	name := l.filename()
	mode := os.FileMode(0600)
	info, err := osStat(name)
	if err == nil {
		// Copy the mode off the old logfile.
		mode = info.Mode()
		// move the existing file
		newname := backupName(name, l.LocalTime)
		if errs := os.Rename(name, newname); errs != nil {
			log.Println("error", errs)
			return fmt.Errorf("can't rename log file: %s", err)
		}

		// this is a no-op anywhere but linux
		if err := chown(name, info); err != nil {
			return err
		}
	}

	// we use truncate here because this should only get called when we've moved
	// the file ourselves. if someone else creates the file in the meantime,
	// just wipe out the contents.
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("can't open new logfile: %s", err)
	}
	l.file = f
	l.size = 0
	return nil
}

func ReplaceFile(
	tempFilePath, currentlyRunningExecutable string,
) (err error) {
	var sb strings.Builder
	sb.WriteString("logs-backup-*-.tmp")

	backupFile, err := os.CreateTemp("", sb.String())
	if err != nil {
		return errors.New("cannot create backup file: %v" + err.Error())
	}
	backupFileName := backupFile.Name()
	_ = backupFile.Close()

	err = atomicReplaceFile(currentlyRunningExecutable, backupFileName)
	if err != nil {
		_ = os.Remove(sb.String())
		return errors.New("cannot rename current executable to backup: %v" + err.Error())
	}

	err = atomicReplaceFile(tempFilePath, currentlyRunningExecutable)
	if err != nil {
		_ = atomicReplaceFile(backupFileName, currentlyRunningExecutable)
		_ = os.Remove(tempFilePath)
		_ = os.Remove(backupFileName)
		return errors.New("cannot rename new file: %v" + err.Error())
	}

	_ = os.Remove(backupFileName)
	return nil
}

const (
	movefile_replace_existing = 0x1
	movefile_write_through    = 0x8
)

//sys moveFileEx(lpExistingFileName *uint16, lpNewFileName *uint16, dwFlags uint32) (err error) = MoveFileExW

// atomicReplaceFile atomically replaces the destination file or directory with the
// source.  It is guaranteed to either replace the target file entirely, or not
// change either file.
func atomicReplaceFile(source, destination string) error {
	src, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return &os.LinkError{Op: "replace", Old: source, New: destination, Err: err}
	}
	dest, err := syscall.UTF16PtrFromString(destination)
	if err != nil {
		return &os.LinkError{Op: "replace", Old: source, New: destination, Err: err}
	}

	// see http://msdn.microsoft.com/en-us/library/windows/desktop/aa365240(v=vs.85).aspx
	if err := moveFileEx(src, dest, movefile_replace_existing|movefile_write_through); err != nil {
		return &os.LinkError{Op: "replace", Old: source, New: destination, Err: err}
	}
	return nil
}

func moveFileEx(lpExistingFileName *uint16, lpNewFileName *uint16, dwFlags uint32) (err error) {
	modkernel32 := syscall.NewLazyDLL("kernel32.dll")
	procMoveFileExW := modkernel32.NewProc("MoveFileExW")

	r1, _, e1 := syscall.SyscallN(
		procMoveFileExW.Addr(),
		uintptr(unsafe.Pointer(lpExistingFileName)),
		uintptr(unsafe.Pointer(lpNewFileName)),
		uintptr(dwFlags),
	)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
