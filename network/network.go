package network

import (
	"fmt"
	"net"

	"github.com/michaelhenkel/gokvm/qemu"
	"gopkg.in/yaml.v3"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type NetworkType string

const (
	BRIDGE NetworkType = "bridge"
	OVS    NetworkType = "ovs"
)

type Network struct {
	Name       string
	Type       NetworkType
	Subnet     *net.IPNet
	Gateway    net.IP
	DNSServer  net.IP
	DHCP       bool
	networkCFG libvirtxml.Network
}

func (n *Network) Create() error {

	networkCFG := libvirtxml.Network{
		Name: n.Name,
	}
	if n.Type == BRIDGE {
		bridge := &libvirtxml.NetworkBridge{
			Name:  n.Name,
			STP:   "on",
			Delay: "0",
		}
		networkCFG.Bridge = bridge
		networkForward := &libvirtxml.NetworkForward{
			Mode: "nat",
			NAT: &libvirtxml.NetworkForwardNAT{
				Ports: []libvirtxml.NetworkForwardNATPort{{
					Start: 1024,
					End:   65535,
				}},
			},
		}
		networkCFG.Forward = networkForward
		networkIPS := []libvirtxml.NetworkIP{{
			Address: n.Gateway.String(),
			Netmask: net.IP(n.Subnet.Mask).String(),
		}}
		if n.DHCP {
			var ips []string
			for ip := n.Subnet.IP.Mask(n.Subnet.Mask); n.Subnet.Contains(ip); inc(ip) {
				ips = append(ips, ip.String())
			}
			networkIPS[0].DHCP = &libvirtxml.NetworkDHCP{
				Ranges: []libvirtxml.NetworkDHCPRange{{
					Start: ips[2],
					End:   ips[len(ips)-2],
				}},
			}
		}
		networkCFG.IPs = networkIPS
		n.networkCFG = networkCFG
	}
	networkXML, err := n.networkCFG.Marshal()
	if err != nil {
		return err
	}
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	_, err = l.NetworkCreateXML(networkXML)
	if err != nil {
		return err
	}

	return nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (n *Network) PrintXML() (string, error) {
	xmlString, err := n.networkCFG.Marshal()
	if err != nil {
		return "", err
	}
	return xmlString, nil
}

func (n *Network) Print() string {
	networkByte, _ := yaml.Marshal(n)
	return fmt.Sprintf("%s\n", string(networkByte))
}
