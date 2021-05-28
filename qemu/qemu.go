package qemu

import (
	libvirt "libvirt.org/libvirt-go"
)

func Connnect() {
	libvirt.NewConnect("")
}
