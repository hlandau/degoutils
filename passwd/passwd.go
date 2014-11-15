// +build unix
package passwd

import "strconv"
import "fmt"
import "unsafe"

/*
#include <sys/types.h>
#include <stdlib.h>
int de_username_to_uid(const char *username, uid_t *uid);
int de_groupname_to_gid(const char *groupname, gid_t *gid);
*/
import "C"

func ParseUID(uid string) (int, error) {
	n, err := strconv.ParseUint(uid, 10, 31)
	if err != nil {
		return parseUserName(uid)
	}
	return int(n), nil
}

func ParseGID(gid string) (int, error) {
	n, err := strconv.ParseUint(gid, 10, 31)
	if err != nil {
		return parseGroupName(gid)
	}
	return int(n), nil
}

func parseUserName(username string) (int, error) {
	var x C.uid_t
	cusername := C.CString(username)
	defer C.free(unsafe.Pointer(cusername))

	if C.de_username_to_uid(cusername, &x) < 0 {
		return 0, fmt.Errorf("cannot convert username to uid: %s", username)
	}
	return int(x), nil
}

func parseGroupName(groupname string) (int, error) {
	var x C.gid_t
	cgroupname := C.CString(groupname)
	defer C.free(unsafe.Pointer(cgroupname))

	if C.de_groupname_to_gid(cgroupname, &x) < 0 {
		return 0, fmt.Errorf("cannot convert group name to gid: %s", groupname)
	}
	return int(x), nil
}
