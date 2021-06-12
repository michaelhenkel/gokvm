package remote

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"

	log "github.com/sirupsen/logrus"
)

type Connection struct {
	*ssh.Client
}

type SignerContainer struct {
	signers []ssh.Signer
}

func (t *SignerContainer) Key(i int) (key ssh.PublicKey, err error) {
	if i >= len(t.signers) {
		return
	}
	key = t.signers[i].PublicKey()
	return
}

func (t *SignerContainer) Sign(i int, rand io.Reader, data []byte) (sig *ssh.Signature, err error) {
	if i >= len(t.signers) {
		return
	}
	sig, err = t.signers[i].Sign(rand, data)
	return
}

func makeSigner(keyname string) (signer ssh.Signer, err error) {
	fp, err := os.Open(keyname)
	if err != nil {
		return
	}
	defer fp.Close()

	buf, _ := ioutil.ReadAll(fp)
	signer, _ = ssh.ParsePrivateKey(buf)
	return
}

func makeKeyring() []ssh.Signer {
	signers := []ssh.Signer{}
	keys := []string{os.Getenv("HOME") + "/.ssh/id_rsa", os.Getenv("HOME") + "/.ssh/id_dsa"}

	for _, keyname := range keys {
		signer, err := makeSigner(keyname)
		if err == nil {
			signers = append(signers, signer)
		}
	}
	return signers
}

func Connect(addr, user string) (*Connection, error) {

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(makeKeyring()...),
		},
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
	}

	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, err
	}

	return &Connection{conn}, nil

}

func GetUbuntuCMDS() []string {
	return []string{
		"hwclock --hctosys",
		"sed -i '/ swap / s/^\\(.*\\)$/#\\1/g' /etc/fstab",
		"swapoff -a",
		"modprobe overlay",
		"modprobe br_netfilter",
		"echo overlay > /etc/modules-load.d/crio.conf",
		"echo br_netfilter >> /etc/modules-load.d/crio.conf",
		"echo \"net.bridge.bridge-nf-call-ip6tables = 1\" > /etc/sysctl.d/kubernetes.conf",
		"echo \"net.bridge.bridge-nf-call-iptables = 1\" >> /etc/sysctl.d/kubernetes.conf",
		"echo \"net.ipv4.ip_forward = 1\" >> /etc/sysctl.d/kubernetes.conf",
		"sysctl --system",
		". /etc/os-release",
		"curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -",
		"wget -nv https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/Release.key -O- | apt-key add -",
		"wget -nv http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/1.20:/1.20.2/x${NAME}_${VERSION_ID}/Release.key -O- | apt-key add -",
		"echo \"deb https://apt.kubernetes.io/ kubernetes-xenial main\" > /etc/apt/sources.list.d/kubernetes.list",
		"echo \"deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/1.20:/1.20.2/x${NAME}_${VERSION_ID}/ /\" > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list",
		"echo \"deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/ /\" >> /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list",
		"apt update",
		"apt -y install kubelet kubeadm kubectl cri-o cri-o-runc",
		"systemctl enable crio",
		"systemctl start crio",
		"systemctl enable kubelet",
		"rm -rf /etc/cni/net.d/*",
	}
}

/*
[4/15 2:21 PM] Michael Henkel

nodeip=<add node ip>
cat <<EOF > kubeadm.yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    node-ip: ${​​​​​​​nodeip}​​​​​​​
EOF
kubeadmin init --cri-socket /var/run/crio.socket



*/

func KubeadmImagePull() []string {
	return []string{
		"kubeadm config images pull --cri-socket /var/run/crio/crio.sock",
	}
}

func KubeadmInit(podCIDR, serviceCIDR, endpoint string) []string {
	return []string{
		fmt.Sprintf("kubeadm init --pod-network-cidr=%s --service-cidr=%s --control-plane-endpoint=%s --cri-socket /var/run/crio/crio.sock", podCIDR, serviceCIDR, endpoint),
	}
}

func (conn *Connection) SendCommands(distro string, cmds []string, wg *sync.WaitGroup) ([]byte, error) {
	if wg != nil {
		defer wg.Done()
	}
	session, err := conn.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO: 0, // disable echoing
		//ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		//ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	err = session.RequestPty("xterm", 80, 40, modes)
	if err != nil {
		return []byte{}, err
	}

	in, err := session.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}

	out, err := session.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	var output []byte
	var done = make(chan bool)
	go func(in io.WriteCloser, out io.Reader, output *[]byte) {
		var (
			line string
			r    = bufio.NewReader(out)
		)
		for {
			b, err := r.ReadByte()
			if err != nil {
				break
			}

			*output = append(*output, b)

			if b == byte('\n') {
				line = ""
				continue
			}

			line += string(b)
		}
		done <- true
	}(in, out, &output)
	cmd := strings.Join(cmds, "; ")
	_, err = session.Output(cmd)
	<-done
	if err != nil {
		return []byte{}, err
	}
	return output, nil

}

/*
hwclock --hctosys
sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
swapoff -a
modprobe overlay
modprobe br_netfilter
tee /etc/sysctl.d/kubernetes.conf<<EOF
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
EOF
sysctl --system

. /etc/os-release
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
wget -nv https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/Release.key -O- | apt-key add -
wget -nv http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/1.20:/1.20.2/x${NAME}_${VERSION_ID}/Release.key -O- | apt-key add -
echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" > /etc/apt/sources.list.d/kubernetes.list
echo "deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable:/cri-o:/1.20:/1.20.2/x${NAME}_${VERSION_ID}/ /" > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
echo "deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_${VERSION_ID}/ /" >> /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
apt update
apt -y install kubelet kubeadm kubectl cri-o
*/
