package passwd
import "strconv"

func ParseUID(uid string) (int, error) {
	n, err := strconv.ParseUint(uid, 10, 31)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func ParseGID(gid string) (int, error) {
	n, err := strconv.ParseUint(gid, 10, 31)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
