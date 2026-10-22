package cmd

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

func checkPlatform() error { return nil }

// harden locks other processes out of this one: its DACL lets this user,
// and the owner (whose implicit rights would otherwise include changing
// the DACL back), only query limited information and wait on it. Without
// PROCESS_VM_READ nobody but an administrator with SeDebugPrivilege can
// read its memory. Handles opened before, such as the one the starting
// process holds, keep their access.
func harden() error {
	us, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	owner, err := windows.StringToSid("S-1-3-4") // OWNER RIGHTS
	if err != nil {
		return err
	}
	const limited = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE
	entry := func(sid *windows.SID, kind windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: limited,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  kind,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		}
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		entry(us.User.Sid, windows.TRUSTEE_IS_USER),
		entry(owner, windows.TRUSTEE_IS_WELL_KNOWN_GROUP),
	}, nil)
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(windows.CurrentProcess(), windows.SE_KERNEL_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

// desktopSession reports why no window can open here, or nil. Services
// run in session 0, which has no desktop, and so does an SSH login.
func desktopSession() error {
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err == nil && session == 0 {
		return errors.New("session 0 has no desktop")
	}
	if os.Getenv("SSH_CONNECTION") != "" {
		return errors.New("an SSH session has no desktop")
	}
	return nil
}
