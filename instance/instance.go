package instance

import (
	"fmt"

	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/metadata"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/qemu"

	"libvirt.org/libvirt-go"

	libvirtxml "libvirt.org/libvirt-go-xml"
)

type Role string

const (
	Controller Role = "controller"
	Worker     Role = "worker"
)

type Instance struct {
	Name        string
	Image       image.Image
	Resources   Resources
	PubKey      string
	DNSServer   string
	Network     network.Network
	ClusterName string
	Suffix      string
	IPAddresses []string
	Role        Role
}

type Resources struct {
	CPU    int
	Memory uint64
	Disk   string
}

func getUintPtr(in uint) *uint {
	return &in
}

func Get(name string, clusterName string) (*Instance, error) {
	instances, err := List(clusterName)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		if inst.Name == name {
			return inst, nil
		}
	}
	return nil, nil
}

func (i *Instance) Delete() error {
	inst, err := Get(i.Name, i.ClusterName)
	if err != nil {
		return err
	}
	if inst != nil {
		l, err := qemu.Connnect()
		if err != nil {
			return err
		}
		domain, err := l.LookupDomainByName(i.Name)
		if err != nil {
			return err
		}
		domainActive, err := domain.IsActive()
		if err != nil {
			return err
		}
		if domainActive {
			if err := domain.Destroy(); err != nil {
				return err
			}
		}
		if err := domain.Undefine(); err != nil {
			return err
		}
		i.Image.Instance = i.Name
		i.Image.Name = "disk"
		i.Image.ImageType = image.INSTANCE
		found, err := i.Image.Get()
		if err != nil {
			return err
		}
		if found {
			if err := i.Image.Delete(); err != nil {
				return err
			}
		}
		i.Image.Instance = i.Name
		i.Image.Name = "cloudinit"
		i.Image.ImageType = image.INSTANCE
		found, err = i.Image.Get()
		if err != nil {
			return err
		}
		if found {
			if err := i.Image.Delete(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Instance) Create() error {
	distributionImage := i.Image
	imgPath := fmt.Sprintf("%s/%s", i.Image.LibvirtImagePath, i.Name)
	i.Image.ImagePath = imgPath
	cloudInitImg, err := i.createCloudInit()
	if err != nil {
		return err
	}
	cloudInitImagePath := cloudInitImg.ImagePath
	i.Image.LibvirtImagePath = distributionImage.LibvirtImagePath
	i.Image.ImagePath = imgPath
	img, err := i.createInstanceImage(distributionImage)
	if err != nil {
		return err
	}
	found, err := i.Image.Get()
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("base image not found")
	}
	role := string(i.Role)
	m := &metadata.Metadata{
		Cluster: &i.ClusterName,
		Suffix:  &i.Suffix,
		Role:    &role,
	}
	domainMetadata := m.InstanceMetadata()
	defaultDomain, err := defaultDomain()
	if err != nil {
		return err
	}
	defaultDomain.Name = i.Name
	defaultDomain.Metadata = &libvirtxml.DomainMetadata{
		XML: domainMetadata,
	}

	defaultDomain.Memory = &libvirtxml.DomainMemory{
		Value: uint(i.Resources.Memory),
		Unit:  "b",
	}
	defaultDomain.VCPU = &libvirtxml.DomainVCPU{
		Placement: "static",
		Value:     uint(i.Resources.CPU),
	}
	defaultDomain.OS = &libvirtxml.DomainOS{
		Type: &libvirtxml.DomainOSType{
			Arch:    "x86_64",
			Machine: "pc-q35-rhel8.2.0",
			Type:    "hvm",
		},
	}

	defaultDomain.CPU = &libvirtxml.DomainCPU{
		Mode:  "custom",
		Match: "exact",
		Check: "full",
	}
	defaultDomain.Devices.Emulator = "/usr/local/bin/qemu-system-x86_64"
	cdrom := libvirtxml.DomainDisk{
		Device: "cdrom",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: cloudInitImagePath,
			},
			Index: 2,
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "sda",
			Bus: "sata",
		},
		ReadOnly: &libvirtxml.DomainDiskReadOnly{},
		Alias: &libvirtxml.DomainAlias{
			Name: "sata0-0-0",
		},
		Address: &libvirtxml.DomainAddress{
			Drive: &libvirtxml.DomainAddressDrive{
				Controller: getUintPtr(0),
				Bus:        getUintPtr(0),
				Target:     getUintPtr(0),
				Unit:       getUintPtr(0),
			},
		},
	}
	defaultDomain.Devices.Disks = append(defaultDomain.Devices.Disks, cdrom)
	disk := libvirtxml.DomainDisk{
		Device: "disk",
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "qcow2",
		},
		Source: &libvirtxml.DomainDiskSource{
			File: &libvirtxml.DomainDiskSourceFile{
				File: img.ImagePath,
			},
		},
		BackingStore: &libvirtxml.DomainDiskBackingStore{
			Format: &libvirtxml.DomainDiskFormat{
				Type: "qcow2",
			},
			Source: &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: distributionImage.ImagePath,
				},
			},
		},
		Target: &libvirtxml.DomainDiskTarget{
			Dev: "vda",
			Bus: "virtio",
		},
		Alias: &libvirtxml.DomainAlias{
			Name: "virtio-disk0",
		},

		Address: &libvirtxml.DomainAddress{
			PCI: &libvirtxml.DomainAddressPCI{
				Domain:   getUintPtr(0),
				Bus:      getUintPtr(3),
				Slot:     getUintPtr(0),
				Function: getUintPtr(0),
			},
		},
	}
	defaultDomain.Devices.Disks = append(defaultDomain.Devices.Disks, disk)
	networkInterface := libvirtxml.DomainInterface{
		Model: &libvirtxml.DomainInterfaceModel{
			Type: "virtio",
		},
		Address: &libvirtxml.DomainAddress{
			PCI: &libvirtxml.DomainAddressPCI{
				Domain:   getUintPtr(0),
				Bus:      getUintPtr(1),
				Slot:     getUintPtr(0),
				Function: getUintPtr(0),
			},
		},
		Source: &libvirtxml.DomainInterfaceSource{
			Network: &libvirtxml.DomainInterfaceSourceNetwork{
				Network: i.Network.Name,
				Bridge:  i.Network.Bridge,
			},
		},
	}
	var domainInterfaces []libvirtxml.DomainInterface
	domainInterfaces = append(domainInterfaces, networkInterface)
	defaultDomain.Devices.Interfaces = domainInterfaces

	domainXML, err := defaultDomain.Marshal()
	if err != nil {
		return err
	}
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	/*
		var testDomain libvirtxml.Domain
		if err := testDomain.Unmarshal(dm); err != nil {
			return err
		}
		testDomXML, err := testDomain.Marshal()
		if err != nil {
			return err
		}
		log.Info("###################TEST#####################")
		log.Info(testDomXML)
		log.Info("###################TEST#####################")
		_, err = l.DomainDefineXML(testDomXML)
		if err != nil {
			return err
		}
	*/
	ldom, err := l.DomainDefineXML(domainXML)
	if err != nil {
		return err
	}
	if err := ldom.SetAutostart(true); err != nil {
		return err
	}
	if err := ldom.Create(); err != nil {
		return err
	}

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
		if md.Cluster == nil {
			continue
		}
		if *md.Cluster != cluster && cluster != "" {
			continue
		}
		var role, suffix string
		if *md.Role != "" {
			role = *md.Role
		}
		if *md.Suffix != "" {
			suffix = *md.Suffix
		}
		inst, err := domainToInstance(domain, *md.Cluster, role, suffix)
		if err != nil {
			return nil, err
		}
		instanceList = append(instanceList, inst)
	}
	return instanceList, nil
}

func domainToInstance(domain libvirt.Domain, cluster string, role string, suffix string) (*Instance, error) {
	instName, err := domain.GetName()
	if err != nil {
		return nil, err
	}
	intfList, err := domain.ListAllInterfaceAddresses(0)
	if err != nil {
		return nil, err
	}
	var ipaddresses []string
	for _, intf := range intfList {
		for _, addr := range intf.Addrs {
			ipaddresses = append(ipaddresses, addr.Addr)
		}
	}

	return &Instance{
		Name:        instName,
		ClusterName: cluster,
		IPAddresses: ipaddresses,
		Role:        Role(role),
		Suffix:      suffix,
	}, nil
}

func defaultDomain() (*libvirtxml.Domain, error) {
	libvirtDomain := &libvirtxml.Domain{}
	if err := libvirtDomain.Unmarshal(domainModel); err != nil {
		return nil, err
	}
	return libvirtDomain, nil
}

var dm string = `<domain type='kvm' id='1'>
<name>cluster1-ubuntu1</name>
<uuid>9dfe0fc3-d1a8-437e-8238-3451cd6fe961</uuid>
<metadata>
  <libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0">
	<libosinfo:os id="http://ubuntu.com/ubuntu/20.04"/>
  </libosinfo:libosinfo>
</metadata>
<memory unit='KiB'>20480000</memory>
<currentMemory unit='KiB'>20480000</currentMemory>
<vcpu placement='static'>4</vcpu>
<resource>
  <partition>/machine</partition>
</resource>
<os>
  <type arch='x86_64' machine='pc-q35-rhel8.2.0'>hvm</type>
  <boot dev='hd'/>
  <bootmenu enable='yes'/>
</os>
<features>
  <acpi/>
  <apic/>
</features>
<cpu mode='custom' match='exact' check='full'>
  <model fallback='forbid'>Haswell-noTSX-IBRS</model>
  <vendor>Intel</vendor>
  <feature policy='require' name='vme'/>
  <feature policy='require' name='ss'/>
  <feature policy='require' name='vmx'/>
  <feature policy='require' name='f16c'/>
  <feature policy='require' name='rdrand'/>
  <feature policy='require' name='hypervisor'/>
  <feature policy='require' name='arat'/>
  <feature policy='require' name='tsc_adjust'/>
  <feature policy='require' name='umip'/>
  <feature policy='require' name='md-clear'/>
  <feature policy='require' name='stibp'/>
  <feature policy='require' name='arch-capabilities'/>
  <feature policy='require' name='ssbd'/>
  <feature policy='require' name='xsaveopt'/>
  <feature policy='require' name='pdpe1gb'/>
  <feature policy='require' name='abm'/>
  <feature policy='require' name='ibpb'/>
  <feature policy='require' name='amd-ssbd'/>
  <feature policy='require' name='skip-l1dfl-vmentry'/>
  <feature policy='require' name='pschange-mc-no'/>
</cpu>
<clock offset='utc'>
  <timer name='rtc' tickpolicy='catchup'/>
  <timer name='pit' tickpolicy='delay'/>
  <timer name='hpet' present='no'/>
</clock>
<on_poweroff>destroy</on_poweroff>
<on_reboot>restart</on_reboot>
<on_crash>destroy</on_crash>
<pm>
  <suspend-to-mem enabled='no'/>
  <suspend-to-disk enabled='no'/>
</pm>
<devices>
  <emulator>/usr/local/bin/qemu-system-x86_64</emulator>
  <disk type='file' device='cdrom'>
	<driver name='qemu' type='raw'/>
	<source file='/var/lib/libvirt/images/cluster1-ubuntu1-seed.img' index='2'/>
	<backingStore/>
	<target dev='sda' bus='sata'/>
	<readonly/>
	<alias name='sata0-0-0'/>
	<address type='drive' controller='0' bus='0' target='0' unit='0'/>
  </disk>
  <disk type='file' device='disk'>
	<driver name='qemu' type='qcow2'/>
	<source file='/var/lib/libvirt/images/ubuntu-20.04-server-cloudimg-amd64.img-cluster1-ubuntu1.qcow2' index='1'/>
	<backingStore type='file' index='3'>
	  <format type='qcow2'/>
	  <source file='/var/lib/libvirt/images/ubuntu-20.04-server-cloudimg-amd64.img'/>
	  <backingStore/>
	</backingStore>
	<target dev='vda' bus='virtio'/>
	<alias name='virtio-disk0'/>
	<address type='pci' domain='0x0000' bus='0x03' slot='0x00' function='0x0'/>
  </disk>
  <controller type='usb' index='0' model='ich9-ehci1'>
	<alias name='usb'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x7'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci1'>
	<alias name='usb'/>
	<master startport='0'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x0' multifunction='on'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci2'>
	<alias name='usb'/>
	<master startport='2'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x1'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci3'>
	<alias name='usb'/>
	<master startport='4'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x2'/>
  </controller>
  <controller type='sata' index='0'>
	<alias name='ide'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1f' function='0x2'/>
  </controller>
  <controller type='pci' index='0' model='pcie-root'>
	<alias name='pcie.0'/>
  </controller>
  <controller type='virtio-serial' index='0'>
	<alias name='virtio-serial0'/>
	<address type='pci' domain='0x0000' bus='0x02' slot='0x00' function='0x0'/>
  </controller>
  <controller type='pci' index='1' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='1' port='0x10'/>
	<alias name='pci.1'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0' multifunction='on'/>
  </controller>
  <controller type='pci' index='2' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='2' port='0x11'/>
	<alias name='pci.2'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x1'/>
  </controller>
  <controller type='pci' index='3' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='3' port='0x12'/>
	<alias name='pci.3'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x2'/>
  </controller>
  <controller type='pci' index='4' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='4' port='0x13'/>
	<alias name='pci.4'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x3'/>
  </controller>
  <controller type='pci' index='5' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='5' port='0x14'/>
	<alias name='pci.5'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x4'/>
  </controller>
  <controller type='pci' index='6' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='6' port='0x15'/>
	<alias name='pci.6'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x5'/>
  </controller>
  <interface type='network'>
	<mac address='52:54:00:60:c2:d3'/>
	<source network='cluster1_network1' portid='2e6307a6-8147-478e-8bf9-19f5b7d03ae4' bridge='virbr2'/>
	<target dev='vnet0'/>
	<model type='virtio'/>
	<alias name='net0'/>
	<address type='pci' domain='0x0000' bus='0x01' slot='0x00' function='0x0'/>
  </interface>
  <serial type='pty'>
	<source path='/dev/pts/1'/>
	<target type='isa-serial' port='0'>
	  <model name='isa-serial'/>
	</target>
	<alias name='serial0'/>
  </serial>
  <console type='pty' tty='/dev/pts/1'>
	<source path='/dev/pts/1'/>
	<target type='serial' port='0'/>
	<alias name='serial0'/>
  </console>
  <channel type='unix'>
	<source mode='bind' path='/var/lib/libvirt/qemu/channel/target/domain-1-cluster1-ubuntu1/org.qemu.guest_agent.0'/>
	<target type='virtio' name='org.qemu.guest_agent.0' state='disconnected'/>
	<alias name='channel0'/>
	<address type='virtio-serial' controller='0' bus='0' port='1'/>
  </channel>
  <input type='tablet' bus='usb'>
	<alias name='input0'/>
	<address type='usb' bus='0' port='1'/>
  </input>
  <input type='mouse' bus='ps2'>
	<alias name='input1'/>
  </input>
  <input type='keyboard' bus='ps2'>
	<alias name='input2'/>
  </input>
  <graphics type='vnc' port='5900' autoport='yes' listen='127.0.0.1'>
	<listen type='address' address='127.0.0.1'/>
  </graphics>
  <video>
	<model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>
	<alias name='video0'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x0'/>
  </video>
  <memballoon model='virtio'>
	<alias name='balloon0'/>
	<address type='pci' domain='0x0000' bus='0x04' slot='0x00' function='0x0'/>
  </memballoon>
  <rng model='virtio'>
	<backend model='random'>/dev/urandom</backend>
	<alias name='rng0'/>
	<address type='pci' domain='0x0000' bus='0x05' slot='0x00' function='0x0'/>
  </rng>
</devices>
<seclabel type='dynamic' model='selinux' relabel='yes'>
  <label>system_u:system_r:svirt_t:s0:c541,c752</label>
  <imagelabel>system_u:object_r:svirt_image_t:s0:c541,c752</imagelabel>
</seclabel>
<seclabel type='dynamic' model='dac' relabel='yes'>
  <label>+107:+107</label>
  <imagelabel>+107:+107</imagelabel>
</seclabel>
</domain>`

var domainModel string = `<domain type='kvm' id='1'>
<name>cluster1-ubuntu1</name>
<memory unit='KiB'>20480000</memory>
<currentMemory unit='KiB'>20480000</currentMemory>
<vcpu placement='static'>4</vcpu>
<resource>
  <partition>/machine</partition>
</resource>
<os>
  <type arch='x86_64' machine='pc-q35-rhel8.2.0'>hvm</type>
  <boot dev='hd'/>
  <bootmenu enable='yes'/>
</os>
<features>
  <acpi/>
  <apic/>
</features>
<clock offset='utc'>
  <timer name='rtc' tickpolicy='catchup'/>
  <timer name='pit' tickpolicy='delay'/>
  <timer name='hpet' present='no'/>
</clock>
<on_poweroff>destroy</on_poweroff>
<on_reboot>restart</on_reboot>
<on_crash>destroy</on_crash>
<pm>
  <suspend-to-mem enabled='no'/>
  <suspend-to-disk enabled='no'/>
</pm>
<devices>
  <controller type='usb' index='0' model='ich9-ehci1'>
	<alias name='usb'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x7'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci1'>
	<alias name='usb'/>
	<master startport='0'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x0' multifunction='on'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci2'>
	<alias name='usb'/>
	<master startport='2'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x1'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci3'>
	<alias name='usb'/>
	<master startport='4'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x2'/>
  </controller>
  <controller type='sata' index='0'>
	<alias name='ide'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1f' function='0x2'/>
  </controller>
  <controller type='pci' index='0' model='pcie-root'>
	<alias name='pcie.0'/>
  </controller>
  <controller type='virtio-serial' index='0'>
	<alias name='virtio-serial0'/>
	<address type='pci' domain='0x0000' bus='0x02' slot='0x00' function='0x0'/>
  </controller>
  <controller type='pci' index='1' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='1' port='0x10'/>
	<alias name='pci.1'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0' multifunction='on'/>
  </controller>
  <controller type='pci' index='2' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='2' port='0x11'/>
	<alias name='pci.2'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x1'/>
  </controller>
  <controller type='pci' index='3' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='3' port='0x12'/>
	<alias name='pci.3'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x2'/>
  </controller>
  <controller type='pci' index='4' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='4' port='0x13'/>
	<alias name='pci.4'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x3'/>
  </controller>
  <controller type='pci' index='5' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='5' port='0x14'/>
	<alias name='pci.5'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x4'/>
  </controller>
  <controller type='pci' index='6' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='6' port='0x15'/>
	<alias name='pci.6'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x5'/>
  </controller>
  <serial type='pty'>
  <target type='isa-serial' port='0'>
	<model name='isa-serial'/>
  </target>
  <alias name='serial0'/>
</serial>
<console type='pty'>
  <source path='/dev/pts/1'/>
  <target type='serial' port='0'/>
  <alias name='serial0'/>
</console>
<channel type='unix'>
  <target type='virtio' name='org.qemu.guest_agent.0' state='disconnected'/>
  <alias name='channel0'/>
  <address type='virtio-serial' controller='0' bus='0' port='1'/>
</channel>
  <input type='tablet' bus='usb'>
	<alias name='input0'/>
	<address type='usb' bus='0' port='1'/>
  </input>
  <input type='mouse' bus='ps2'>
	<alias name='input1'/>
  </input>
  <input type='keyboard' bus='ps2'>
	<alias name='input2'/>
  </input>
  <graphics type='vnc' port='-1' autoport='yes'>
  <listen type='address'/>
</graphics>
  <video>
	<model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>
	<alias name='video0'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x0'/>
  </video>
  <memballoon model='virtio'>
	<alias name='balloon0'/>
	<address type='pci' domain='0x0000' bus='0x04' slot='0x00' function='0x0'/>
  </memballoon>
  <rng model='virtio'>
	<backend model='random'>/dev/urandom</backend>
	<alias name='rng0'/>
	<address type='pci' domain='0x0000' bus='0x05' slot='0x00' function='0x0'/>
  </rng>
</devices>
<seclabel type='dynamic' model='dac' relabel='yes'>
  <label>+107:+107</label>
  <imagelabel>+107:+107</imagelabel>
</seclabel>
</domain>`
