package qemu

import (
	"fmt"
	"net"
	"time"

	libvirt "github.com/digitalocean/go-libvirt"
)

func Connnect() (*libvirt.Libvirt, error) {
	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial libvirt: %v", err)
	}
	l := libvirt.New(c)
	if err := l.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	return l, nil

}
