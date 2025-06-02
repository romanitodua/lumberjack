//go:build linux
// +build linux

package lamberjack

import (
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestMaintainMode(t *testing.T) {
	lumberjack.currentTime = lumberjack.fakeTime
	dir := lumberjack.makeTempDir("TestMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := lumberjack.logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	lumberjack.isNil(err, t)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	lumberjack.isNil(err, t)
	lumberjack.equals(len(b), n, t)

	lumberjack.newFakeTime()

	err = l.Rotate()
	lumberjack.isNil(err, t)

	filename2 := lumberjack.backupFile(dir)
	info, err := os.Stat(filename)
	lumberjack.isNil(err, t)
	info2, err := os.Stat(filename2)
	lumberjack.isNil(err, t)
	lumberjack.equals(mode, info.Mode(), t)
	lumberjack.equals(mode, info2.Mode(), t)
}

func TestMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	lumberjack.osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		lumberjack.osStat = os.Stat
	}()
	lumberjack.currentTime = lumberjack.fakeTime
	dir := lumberjack.makeTempDir("TestMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := lumberjack.logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	lumberjack.isNil(err, t)
	f.Close()

	l := &Logger{
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	lumberjack.isNil(err, t)
	lumberjack.equals(len(b), n, t)

	lumberjack.newFakeTime()

	err = l.Rotate()
	lumberjack.isNil(err, t)

	lumberjack.equals(555, fakeFS.files[filename].uid, t)
	lumberjack.equals(666, fakeFS.files[filename].gid, t)
}

func TestCompressMaintainMode(t *testing.T) {
	lumberjack.currentTime = lumberjack.fakeTime

	dir := lumberjack.makeTempDir("TestCompressMaintainMode", t)
	defer os.RemoveAll(dir)

	filename := lumberjack.logFile(dir)

	mode := os.FileMode(0600)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, mode)
	lumberjack.isNil(err, t)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	lumberjack.isNil(err, t)
	lumberjack.equals(len(b), n, t)

	lumberjack.newFakeTime()

	err = l.Rotate()
	lumberjack.isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// mode.
	filename2 := lumberjack.backupFile(dir)
	info, err := os.Stat(filename)
	lumberjack.isNil(err, t)
	info2, err := os.Stat(filename2 + lumberjack.compressSuffix)
	lumberjack.isNil(err, t)
	lumberjack.equals(mode, info.Mode(), t)
	lumberjack.equals(mode, info2.Mode(), t)
}

func TestCompressMaintainOwner(t *testing.T) {
	fakeFS := newFakeFS()
	osChown = fakeFS.Chown
	lumberjack.osStat = fakeFS.Stat
	defer func() {
		osChown = os.Chown
		lumberjack.osStat = os.Stat
	}()
	lumberjack.currentTime = lumberjack.fakeTime
	dir := lumberjack.makeTempDir("TestCompressMaintainOwner", t)
	defer os.RemoveAll(dir)

	filename := lumberjack.logFile(dir)

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0644)
	lumberjack.isNil(err, t)
	f.Close()

	l := &Logger{
		Compress:   true,
		Filename:   filename,
		MaxBackups: 1,
		MaxSize:    100, // megabytes
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	lumberjack.isNil(err, t)
	lumberjack.equals(len(b), n, t)

	lumberjack.newFakeTime()

	err = l.Rotate()
	lumberjack.isNil(err, t)

	// we need to wait a little bit since the files get compressed on a different
	// goroutine.
	<-time.After(10 * time.Millisecond)

	// a compressed version of the log file should now exist with the correct
	// owner.
	filename2 := lumberjack.backupFile(dir)
	lumberjack.equals(555, fakeFS.files[filename2+lumberjack.compressSuffix].uid, t)
	lumberjack.equals(666, fakeFS.files[filename2+lumberjack.compressSuffix].gid, t)
}

type fakeFile struct {
	uid int
	gid int
}

type fakeFS struct {
	files map[string]fakeFile
}

func newFakeFS() *fakeFS {
	return &fakeFS{files: make(map[string]fakeFile)}
}

func (fs *fakeFS) Chown(name string, uid, gid int) error {
	fs.files[name] = fakeFile{uid: uid, gid: gid}
	return nil
}

func (fs *fakeFS) Stat(name string) (os.FileInfo, error) {
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	stat := info.Sys().(*syscall.Stat_t)
	stat.Uid = 555
	stat.Gid = 666
	return info, nil
}
