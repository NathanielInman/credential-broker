package keyring

import (
	"errors"
	"net"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sioAFUnixGetPeerPID is SIO_AF_UNIX_GETPEERPID: _WSAIOR(IOC_VENDOR, 256).
const sioAFUnixGetPeerPID = 0x58000100

// peerIsUs checks the process at the other end of conn runs as this user,
// by comparing its token's user SID with ours.
func peerIsUs(conn net.Conn) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New("not a unix socket")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var pid uint32
	var ioErr error
	if err := raw.Control(func(fd uintptr) {
		var n uint32
		ioErr = windows.WSAIoctl(windows.Handle(fd), sioAFUnixGetPeerPID, nil, 0,
			(*byte)(unsafe.Pointer(&pid)), uint32(unsafe.Sizeof(pid)), &n, nil, 0)
	}); err != nil {
		return err
	}
	if ioErr != nil {
		return ioErr
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok); err != nil {
		return err
	}
	defer tok.Close()
	them, err := tok.GetTokenUser()
	if err != nil {
		return err
	}
	us, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	if !windows.EqualSid(them.User.Sid, us.User.Sid) {
		return errors.New("peer is another user")
	}
	return nil
}

// refused reports a socket file with nobody listening.
func refused(err error) bool { return errors.Is(err, windows.WSAECONNREFUSED) }

// detached starts the agent in a process group of its own with a hidden
// console, so closing the caller's console does not end it and the age
// plugins it runs get no window of their own.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
		HideWindow:    true,
	}
}
