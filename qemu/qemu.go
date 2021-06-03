package qemu

import (
	libvirt "libvirt.org/libvirt-go"
)

func Connnect() (*libvirt.Connect, error) {
	conn, err := libvirt.NewConnect("qemu:///system")
	if err != nil {
		return nil, err
	}
	return conn, nil

}
