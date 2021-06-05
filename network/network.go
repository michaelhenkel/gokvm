package network

import (
	"fmt"
	"net"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/metadata"
	"github.com/michaelhenkel/gokvm/qemu"
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

func DefaultNetwork() Network {
	cidr := "192.168.66.0/24"
	gateway := net.ParseIP("192.168.66.1")
	dnsserver := net.ParseIP("192.168.66.1")
	_, subnet, _ := net.ParseCIDR(cidr)
	return Network{
		Name:      "gokvm",
		Type:      BRIDGE,
		Subnet:    subnet,
		Gateway:   gateway,
		DNSServer: dnsserver,
		DHCP:      true,
	}
}

type Network struct {
	Name       string
	Type       NetworkType
	Subnet     *net.IPNet
	Gateway    net.IP
	DNSServer  net.IP
	DHCP       bool
	networkCFG libvirtxml.Network
	Active     bool
	Bridge     string
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

func Get(networkName string) (*Network, error) {
	networks, err := List()
	if err != nil {
		return nil, err
	}
	for _, netw := range networks {
		if netw.Name == networkName {
			return netw, nil
		}
	}
	return nil, nil
}

func List() ([]*Network, error) {
	conn, err := qemu.Connnect()
	if err != nil {
		return nil, err
	}
	networks := []*Network{}

	activeNetworks, err := conn.ListAllNetworks(2)
	if err != nil {
		return nil, err
	}
	for _, anet := range activeNetworks {
		if ok, err := checkMetadata(anet); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		netw, err := lnetworkToNetwork(anet)
		if err != nil {
			return nil, err
		}
		networks = append(networks, netw)
	}
	inActiveNetworks, err := conn.ListAllNetworks(1)
	if err != nil {
		return nil, err
	}
	for _, anet := range inActiveNetworks {
		if ok, err := checkMetadata(anet); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		netw, err := lnetworkToNetwork(anet)
		if err != nil {
			return nil, err
		}
		networks = append(networks, netw)
	}

	return networks, nil
}

func lnetworkToNetwork(lnetwork libvirt.Network) (*Network, error) {
	networkName, err := lnetwork.GetName()
	if err != nil {
		return nil, err
	}
	isActive, err := lnetwork.IsActive()
	if err != nil {
		return nil, err
	}
	networkXML, err := lnetwork.GetXMLDesc(0)
	if err != nil {
		return nil, err
	}
	var xmlNetwork libvirtxml.Network
	if err := xmlNetwork.Unmarshal(networkXML); err != nil {
		return nil, err
	}
	netw := &Network{
		Name:   networkName,
		Active: isActive,
	}
	for _, netwIP := range xmlNetwork.IPs {
		addr := net.ParseIP(netwIP.Netmask).To4()
		sz, _ := net.IPv4Mask(addr[0], addr[1], addr[2], addr[3]).Size()
		ip, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", netwIP.Address, sz))
		if err != nil {
			return nil, err
		}
		netw.Subnet = ipNet
		netw.Gateway = ip
		if netwIP.DHCP != nil {
			netw.DHCP = true
		}
	}
	if xmlNetwork.DNS != nil {
		for _, fw := range xmlNetwork.DNS.Forwarders {
			netw.DNSServer = net.ParseIP(fw.Addr)
		}
	}
	if xmlNetwork.Bridge != nil {
		netw.Bridge = xmlNetwork.Bridge.Name
	}

	return netw, nil
}

func Render(networks []*Network) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Network", "Active"})
	var tableRows []table.Row
	for _, netw := range networks {
		tableRows = append(tableRows, table.Row{netw.Name, netw.Active})
	}
	t.AppendRows(tableRows)
	t.SetStyle(table.StyleLight)
	t.Render()
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
	if xmlNetwork.Metadata != nil {
		md, err := metadata.GetMetadata(xmlNetwork.Metadata.XML)
		if err != nil {
			return false, err
		}
		if md.Net == nil {
			return false, nil
		}
		if *md.Net != "gokvm" {
			return false, nil
		}
		return true, nil
	}
	return false, nil
}

func (n *Network) Create() error {
	conn, err := qemu.Connnect()
	if err != nil {
		return err
	}

	_, err = conn.LookupNetworkByName(n.Name)
	if err == nil {
		return nil
	}

	md := metadata.Metadata{
		Net: &n.Name,
	}
	mdXML := md.InstanceMetadata()
	networkCFG := libvirtxml.Network{
		Name: n.Name,
		Metadata: &libvirtxml.NetworkMetadata{
			XML: mdXML,
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
