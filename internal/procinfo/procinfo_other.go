//go:build !linux && !darwin && !windows

package procinfo

func parent(int) (string, int, error) { return "", 0, ErrUnsupported }

func Exe(int) (string, error) { return "", ErrUnsupported }
func Cwd(int) (string, error) { return "", ErrUnsupported }
