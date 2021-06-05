package instance

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/kdomanski/iso9660"
	"github.com/michaelhenkel/gokvm/image"
	"gopkg.in/yaml.v3"

	log "github.com/sirupsen/logrus"
)

func (i *Instance) createCloudInit() (*image.Image, error) {
	ci := cloudInit{
		Hostname:       i.Name,
		ManageEtcHosts: true,
		Users: []user{{
			Name:              "gokvm",
			Sudo:              "ALL=(ALL) NOPASSWD:ALL",
			Home:              "/home/gokvm",
			Shell:             "/bin/bash",
			LockPasswd:        false,
			SSHAuthorizedKeys: []string{i.PubKey},
		}, {
			Name:              "root",
			SSHAuthorizedKeys: []string{i.PubKey},
		}},
		SSHPwauth:   true,
		DisableRoot: false,
		Chpasswd: chpasswd{
			List: `gokvm:gokvm
root:gokvm`,
			Expire: false,
		},
		WriteFiles: []writeFiles{{
			Content: `[Resolve]
DNS=` + i.Network.DNSServer.String(),
			Path: "/etc/systemd/resolved.conf",
		}},
		RunCMD: []string{
			"systemctl restart systemd-resolved.service",
			"cat /etc/systemd/resolved.conf > /run/test",
		},
	}
	log.Info("Creating ISO")
	out, err := ioutil.TempDir("/tmp", "prefix")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(out)

	cloudInitPath := out + "/config"
	metaDataPath := cloudInitPath + "/meta-data"
	userDataPath := cloudInitPath + "/user-data"
	if err := os.Mkdir(cloudInitPath, 0755); err != nil {
		return nil, err
	}

	ciByte, err := yaml.Marshal(&ci)
	if err != nil {
		return nil, err
	}

	userDataHeader := fmt.Sprintf("#cloud-config\n%s", string(ciByte))
	userDataOut := []byte(userDataHeader)

	if err := os.WriteFile(userDataPath, userDataOut, 0600); err != nil {
		return nil, err
	}

	defaultMetaData := &metaData{
		InstanceId:    ci.Hostname,
		LocalHostname: ci.Hostname,
	}

	metaDataYAML, err := yaml.Marshal(defaultMetaData)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(metaDataPath, metaDataYAML, 0600); err != nil {
		return nil, err
	}

	writer, err := iso9660.NewWriter()
	if err != nil {
		return nil, err
	}
	defer writer.Cleanup()

	userData, err := os.Open(out + "/config/user-data")
	if err != nil {
		return nil, err
	}
	defer userData.Close()

	err = writer.AddFile(userData, "user-data")
	if err != nil {
		return nil, err
	}

	metaData, err := os.Open(out + "/config/meta-data")
	if err != nil {
		return nil, err
	}
	defer metaData.Close()

	err = writer.AddFile(metaData, "meta-data")
	if err != nil {
		return nil, err
	}

	outputFile, err := os.OpenFile(out+"/cidata.iso", os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}

	err = writer.WriteTo(outputFile, "CIDATA")
	if err != nil {
		return nil, err
	}

	err = outputFile.Close()
	if err != nil {
		return nil, err
	}

	img := &image.Image{
		Pool:              "gokvm",
		Name:              fmt.Sprintf("%s-cloudinit", i.Name),
		ImageLocationType: image.File,
		ImageLocation:     out + "/cidata.iso",
	}
	if err := img.Create(); err != nil {
		return nil, err
	}
	log.Info("Created ISO")

	return img, nil
}

type cloudInit struct {
	Hostname       string       `yaml:"hostname"`
	ManageEtcHosts bool         `yaml:"manage_etc_hosts"`
	Users          []user       `yaml:"users"`
	SSHPwauth      bool         `yaml:"ssh_pwauth"`
	DisableRoot    bool         `yaml:"disable_root"`
	Chpasswd       chpasswd     `yaml:"chpasswd"`
	WriteFiles     []writeFiles `yaml:"write_files"`
	RunCMD         []string     `yaml:"runcmd"`
}

type chpasswd struct {
	List   string `yaml:"list"`
	Expire bool   `yaml:"expire"`
}

type writeFiles struct {
	Content string `yaml:"content"`
	Path    string `yaml:"path"`
}

type user struct {
	Name              string   `yaml:"name"`
	Sudo              string   `yaml:"sudo"`
	Groups            string   `yaml:"groups"`
	Home              string   `yaml:"home"`
	Shell             string   `yaml:"shell"`
	LockPasswd        bool     `yaml:"lock_passwd"`
	SSHAuthorizedKeys []string `yaml:"ssh-authorized-keys"`
}

type metaData struct {
	InstanceId    string `yaml:"instance-id"`
	LocalHostname string `yaml:"local-hostname"`
}

/*
#cloud-config
hostname: ${hostname}.${clusterName}.${suffix}
manage_etc_hosts: true
users:
  - name: ubuntu
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: users, admin
    home: /home/ubuntu
    shell: /bin/bash
    lock_passwd: false
    ssh-authorized-keys:
      - ${pubKey}
  - name: root
    ssh-authorized-keys:
      - ${pubKey}
# only cert auth via ssh (console access can still login)
ssh_pwauth: true
disable_root: false
chpasswd:
  list: |
     ubuntu:linux
     root:linux
  expire: False
write_files:
- content: |
    [Resolve]
    DNS=${dnsserver}
  path: /etc/systemd/resolved.conf
runcmd:
  - [ systemctl, restart, systemd-resolved.service ]
  - cat /etc/systemd/resolved.conf > /run/test
*/
