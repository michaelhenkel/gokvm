package network

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/qemu"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	libvirt "libvirt.org/libvirt-go"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type NetworkType string

const (
	BRIDGE          NetworkType = "bridge"
	OVS             NetworkType = "ovs"
	NetworkMetadata string      = `<gokvm:net xmlns:gokvm="http://gokvm">gokvm</gokvm:net>`
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

func (n *Network) Delete() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}

	networkCFG, err := l.LookupNetworkByName(n.Name)
	if err != nil {
		lerr, ok := err.(libvirt.Error)
		if !ok {
			return err
		}
		if lerr.Code == libvirt.ERR_NO_NETWORK {
			log.Info("Network doesn't exist")
			return nil
		}
		return err
	}
	isActive, err := networkCFG.IsActive()
	if err != nil {
		return err
	}
	if isActive {

		if err := networkCFG.Destroy(); err != nil {
			return err
		}
	}
	if err := networkCFG.Undefine(); err != nil {
		return err
	}
	return nil
}

func (n *Network) List() error {
	conn, err := qemu.Connnect()
	if err != nil {
		return err
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Network", "Active"})
	var tableRows []table.Row
	activeNetworks, err := conn.ListAllNetworks(2)
	if err != nil {
		return err
	}
	for _, anet := range activeNetworks {
		if ok, err := checkMetadata(anet); err != nil {
			return err
		} else if !ok {
			continue
		}
		name, err := anet.GetName()
		if err != nil {
			return err
		}
		tableRows = append(tableRows, table.Row{name, "True"})
	}
	inActiveNetworks, err := conn.ListAllNetworks(1)
	if err != nil {
		return err
	}
	for _, anet := range inActiveNetworks {
		if ok, err := checkMetadata(anet); err != nil {
			return err
		} else if !ok {
			continue
		}
		name, err := anet.GetName()
		if err != nil {
			return err
		}
		tableRows = append(tableRows, table.Row{name, "False"})
	}
	t.AppendRows(tableRows)
	t.SetStyle(table.StyleColoredBlackOnBlueWhite)
	t.Render()
	return nil
}

func checkMetadata(lnet libvirt.Network) (bool, error) {
	xmlDesc, err := lnet.GetXMLDesc(0)
	if err != nil {
		return false, err
	}
	var xmlNetwork libvirtxml.Network
	if err := xmlNetwork.Unmarshal(xmlDesc); err != nil {
		return false, err
	}

	if xmlNetwork.Metadata == nil {
		return false, nil
	}
	if strings.TrimSpace(xmlNetwork.Metadata.XML) != NetworkMetadata {
		return false, nil
	}
	return true, nil
}

func (n *Network) Create() error {
	conn, err := qemu.Connnect()
	if err != nil {
		return err
	}

	_, err = conn.LookupNetworkByName(n.Name)
	if err == nil {
		log.Info("Network already exists")
		return nil
	}

	networkCFG := libvirtxml.Network{
		Name: n.Name,
		Metadata: &libvirtxml.NetworkMetadata{
			XML: NetworkMetadata,
		},
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
	libvirtNet, err := conn.NetworkDefineXML(networkXML)
	if err != nil {
		return err
	}
	if err := libvirtNet.Create(); err != nil {
		return err
	}
	if err := libvirtNet.SetAutostart(true); err != nil {
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
