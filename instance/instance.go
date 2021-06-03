package instance

import (
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/metadata"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/qemu"

	"libvirt.org/libvirt-go"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type Instance struct {
	Name        string
	Image       image.Image
	CPU         int
	Memory      int
	Disk        string
	PubKey      string
	DNSServer   string
	Network     network.Network
	ClusterName string
	Suffix      string
}

func (i *Instance) Create() error {
	i.createCloudInit()
	return nil
}

func List(cluster string) ([]*Instance, error) {
	l, err := qemu.Connnect()
	if err != nil {
		return nil, err
	}
	domains, err := l.ListAllDomains(0)
	if err != nil {
		return nil, err
	}
	var instanceList []*Instance
	for _, domain := range domains {
		domainXML, err := domain.GetXMLDesc(0)
		if err != nil {
			return nil, err
		}
		var xmlDomain libvirtxml.Domain
		if err := xmlDomain.Unmarshal(domainXML); err != nil {
			return nil, err
		}
		if xmlDomain.Metadata == nil {
			continue
		}
		md, err := metadata.GetMetadata(xmlDomain.Metadata.XML)
		if err != nil {
			return nil, err
		}
		if md.Cluster != cluster {
			continue
		}
		inst, err := domainToInstance(domain)
		if err != nil {
			return nil, err
		}
		instanceList = append(instanceList, inst)

	}
	return instanceList, nil
}

func domainToInstance(domain libvirt.Domain) (*Instance, error) {
	instName, err := domain.GetName()
	if err != nil {
		return nil, err
	}

	return &Instance{
		Name: instName,
	}, nil
}
