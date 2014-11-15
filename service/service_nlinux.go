// +build !linux

package service

// Dummy implementation
func (h *ihandler) dropPrivilegesExtra() error {
	return nil
}
