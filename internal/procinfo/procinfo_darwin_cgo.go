//go:build darwin && cgo

package procinfo

/*
#include <libproc.h>
#include <sys/proc_info.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Exe is the absolute path of pid's executable (proc_pidpath).
func Exe(pid int) (string, error) {
	buf := make([]byte, C.PROC_PIDPATHINFO_MAXSIZE)
	n := C.proc_pidpath(C.int(pid), unsafe.Pointer(&buf[0]), C.uint32_t(len(buf)))
	if n <= 0 {
		return "", errors.New("proc_pidpath failed")
	}
	return string(buf[:n]), nil
}

// Cwd is pid's working directory (proc_pidinfo PROC_PIDVNODEPATHINFO).
func Cwd(pid int) (string, error) {
	var vpi C.struct_proc_vnodepathinfo
	size := C.int(unsafe.Sizeof(vpi))
	if C.proc_pidinfo(C.int(pid), C.PROC_PIDVNODEPATHINFO, 0, unsafe.Pointer(&vpi), size) != size {
		return "", errors.New("proc_pidinfo failed")
	}
	return C.GoString(&vpi.pvi_cdir.vip_path[0]), nil
}
