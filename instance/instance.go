package instance

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/go-ini/ini"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/metadata"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/qemu"
	"github.com/michaelhenkel/gokvm/snapshot"
	"github.com/vbauerster/mpb/v7"

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
		if err := i.DeleteSnapshot(); err != nil {
			return err
		}
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

func (i *Instance) CreateSnapshot() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	domain, err := l.LookupDomainByName(i.Name)
	if err != nil {
		return err
	}
	domainXML, err := domain.GetXMLDesc(0)
	if err != nil {
		return err
	}
	xmlDomain := &libvirtxml.Domain{}
	if err := xmlDomain.Unmarshal(domainXML); err != nil {
		return err
	}
	ds := &libvirtxml.DomainSnapshot{
		Name:   i.Name,
		Domain: xmlDomain,
	}
	dsXML, err := ds.Marshal()
	if err != nil {
		return err
	}
	_, err = domain.CreateSnapshotXML(dsXML, 0)
	if err != nil {
		return err
	}
	return nil
}

func (i *Instance) ListSnapshot() ([]*snapshot.Snapshot, error) {
	l, err := qemu.Connnect()
	if err != nil {
		return nil, err
	}
	domain, err := l.LookupDomainByName(i.Name)
	if err != nil {
		return nil, err
	}
	snapshotList, err := domain.SnapshotListNames(0)
	if err != nil {
		return nil, err
	}

	snapShotList := []*snapshot.Snapshot{}
	for _, ss := range snapshotList {
		snap, err := domain.SnapshotLookupByName(ss, 0)
		if err != nil {
			return nil, err
		}
		snapshotName, err := snap.GetName()
		if err != nil {
			return nil, err
		}
		isCurrent, err := snap.IsCurrent(0)
		if err != nil {
			return nil, err
		}
		snapShot := &snapshot.Snapshot{
			Name:      snapshotName,
			Instance:  i.Name,
			IsCurrent: isCurrent,
		}
		snapShotList = append(snapShotList, snapShot)
	}
	return snapShotList, nil
}

func (i *Instance) DeleteSnapshot() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	domain, err := l.LookupDomainByName(i.Name)
	if err != nil {
		return err
	}
	snapshotList, err := domain.SnapshotListNames(0)
	if err != nil {
		return err
	}
	for _, ss := range snapshotList {
		sss, err := domain.SnapshotLookupByName(ss, 0)
		if err != nil {
			return err
		}
		if err := sss.Delete(0); err != nil {
			return err
		}
	}
	return nil
}

func (i *Instance) RevertSnapshot() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	domain, err := l.LookupDomainByName(i.Name)
	if err != nil {
		return err
	}
	currentSnapshot, err := domain.SnapshotCurrent(0)
	if err != nil {
		return err
	}
	if err := currentSnapshot.RevertToSnapshot(0); err != nil {
		return err
	}
	return nil
}

func getOSRelease() (map[string]string, error) {
	cfg, err := ini.Load("/etc/os-release")
	if err != nil {
		return nil, err
	}
	configParams := make(map[string]string)
	configParams["VERSION_ID"] = cfg.Section("").Key("VERSION_ID").String()
	configParams["ID"] = cfg.Section("").Key("ID").String()
	return configParams, nil
}

func (i *Instance) Create(bar *mpb.Bar) error {
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
	var machineType string
	osReleaseMap, err := getOSRelease()
	if err != nil {
		return err
	}
	var slot uint
	switch osReleaseMap["ID"] {
	case "centos":
		switch osReleaseMap["VERSION_ID"] {
		case "8":
			machineType = "pc-q35-rhel8.2.0"
			slot = 0
		}
	case "ubuntu":
		switch osReleaseMap["VERSION_ID"] {
		case "20.04":
			machineType = "pc-q35-focal"
			slot = 0
		}
	}
	dd, err := defaultDomain(osReleaseMap["ID"])
	if err != nil {
		return err
	}

	dd.Name = i.Name
	dd.Metadata = &libvirtxml.DomainMetadata{
		XML: domainMetadata,
	}

	dd.Memory = &libvirtxml.DomainMemory{
		Value: uint(i.Resources.Memory),
		Unit:  "b",
	}
	dd.VCPU = &libvirtxml.DomainVCPU{
		Placement: "static",
		Value:     uint(i.Resources.CPU),
	}

	dd.OS = &libvirtxml.DomainOS{
		Type: &libvirtxml.DomainOSType{
			Arch:    "x86_64",
			Machine: machineType,
			Type:    "hvm",
		},
	}

	dd.CPU = &libvirtxml.DomainCPU{
		Mode:  "host-model",
		Check: "none",
	}
	emulatorPath, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return err
	}
	dd.Devices.Emulator = emulatorPath
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
	dd.Devices.Disks = append(dd.Devices.Disks, cdrom)
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
				Slot:     getUintPtr(slot),
				Function: getUintPtr(0),
			},
		},
	}
	dd.Devices.Disks = append(dd.Devices.Disks, disk)
	networkInterface := libvirtxml.DomainInterface{
		Model: &libvirtxml.DomainInterfaceModel{
			Type: "virtio",
		},
		Address: &libvirtxml.DomainAddress{
			PCI: &libvirtxml.DomainAddressPCI{
				Domain:   getUintPtr(0),
				Bus:      getUintPtr(1),
				Slot:     getUintPtr(slot),
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
	dd.Devices.Interfaces = domainInterfaces

	domainXML, err := dd.Marshal()
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
	bar.Increment()
	ipList, err := waitForIPAddress(i.Name, l)
	if err != nil {
		return err
	}
	bar.Increment()
	ipAddress := ipList[0]
	m.IPAddress = &ipAddress
	if err := ldom.SetMetadata(2, "<ipaddress>"+ipAddress+"</ipaddress>", "ipaddress", "http://ipaddress/ipaddress", 0); err != nil {
		return err
	}

	return nil
}

func waitForIPAddress(domainName string, l *libvirt.Connect) ([]string, error) {
	domain, err := l.LookupDomainByName(domainName)
	if err != nil {
		return nil, err
	}
	var ipaddresses []string
	for {
		intfList, err := domain.ListAllInterfaceAddresses(0)
		if err != nil {
			return nil, err
		}
		if len(intfList) > 0 {
			for _, intf := range intfList {
				for _, addr := range intf.Addrs {
					ipaddresses = append(ipaddresses, addr.Addr)
				}
			}
			return ipaddresses, nil
		} else {
			time.Sleep(time.Second * 2)
		}
	}
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
	ipaddressWithTag, err := domain.GetMetadata(2, "http://ipaddress/ipaddress", 0)
	if err != nil {
		fmt.Println("cannot read ip address from instance")
	}
	re := regexp.MustCompile(`<ipaddress.*?>(.*)</ipaddress>`)
	var ipaddress string
	submatchall := re.FindAllStringSubmatch(ipaddressWithTag, -1)
	for _, element := range submatchall {
		ipaddress = element[1]
	}
	return &Instance{
		Name:        instName,
		ClusterName: cluster,
		IPAddresses: []string{ipaddress},
		Role:        Role(role),
		Suffix:      suffix,
	}, nil
}

func defaultDomain(distro string) (*libvirtxml.Domain, error) {
	var domainModel string

	libvirtDomain := &libvirtxml.Domain{}
	switch distro {
	case "centos":
		domainModel = domainModelCentos
	case "ubuntu":
		domainModel = domainModelUbuntu
	}
	if err := libvirtDomain.Unmarshal(domainModel); err != nil {
		return nil, err
	}
	return libvirtDomain, nil
}

var domainModelCentos string = `<domain type='kvm' id='1'>
<name>cluster1-ubuntu1</name>
<memory unit='KiB'>20480000</memory>
<currentMemory unit='KiB'>20480000</currentMemory>
<vcpu placement='static'>4</vcpu>
<resource>
  <partition>/machine</partition>
</resource>
<cpu mode='host-model' check='partial'/>
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

var domainModelUbuntu string = `<domain type='kvm'>
<name>vm1</name>
<memory unit='KiB'>4096</memory>
<currentMemory unit='KiB'>4096</currentMemory>
<vcpu placement='static'>1</vcpu>
<os>
  <type arch='x86_64' machine='pc-q35-focal'>hvm</type>
  <boot dev='hd'/>
</os>
<features>
  <acpi/>
  <apic/>
  <vmport state='off'/>
</features>
<cpu mode='host-model' check='partial'/>
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
  <emulator>/usr/bin/qemu-system-x86_64</emulator>
  <controller type='usb' index='0' model='ich9-ehci1'>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x7'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci1'>
	<master startport='0'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x0' multifunction='on'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci2'>
	<master startport='2'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x1'/>
  </controller>
  <controller type='usb' index='0' model='ich9-uhci3'>
	<master startport='4'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1d' function='0x2'/>
  </controller>
  <controller type='sata' index='0'>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1f' function='0x2'/>
  </controller>
  <controller type='pci' index='0' model='pcie-root'/>
  <controller type='virtio-serial' index='0'>
	<address type='pci' domain='0x0000' bus='0x02' slot='0x00' function='0x0'/>
  </controller>
  <controller type='pci' index='1' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='1' port='0x10'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x0' multifunction='on'/>
  </controller>
  <controller type='pci' index='2' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='2' port='0x11'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x1'/>
  </controller>
  <controller type='pci' index='3' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='3' port='0x12'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x2'/>
  </controller>
  <controller type='pci' index='4' model='pcie-root-port'>
	<model name='pcie-root-port'/>
	<target chassis='4' port='0x13'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x02' function='0x3'/>
  </controller>
  <serial type='pty'>
	<target type='isa-serial' port='0'>
	  <model name='isa-serial'/>
	</target>
  </serial>
  <console type='pty'>
	<target type='serial' port='0'/>
  </console>
  <channel type='spicevmc'>
	<target type='virtio' name='com.redhat.spice.0'/>
	<address type='virtio-serial' controller='0' bus='0' port='1'/>
  </channel>
  <input type='tablet' bus='usb'>
	<address type='usb' bus='0' port='1'/>
  </input>
  <input type='mouse' bus='ps2'/>
  <input type='keyboard' bus='ps2'/>
  <graphics type='spice' autoport='yes'>
	<listen type='address'/>
	<image compression='off'/>
  </graphics>
  <sound model='ich9'>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x1b' function='0x0'/>
  </sound>
  <video>
	<model type='qxl' ram='65536' vram='65536' vgamem='16384' heads='1' primary='yes'/>
	<address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x0'/>
  </video>
  <redirdev bus='usb' type='spicevmc'>
	<address type='usb' bus='0' port='2'/>
  </redirdev>
  <redirdev bus='usb' type='spicevmc'>
	<address type='usb' bus='0' port='3'/>
  </redirdev>
  <memballoon model='virtio'>
	<address type='pci' domain='0x0000' bus='0x04' slot='0x00' function='0x0'/>
  </memballoon>
</devices>
</domain>`
