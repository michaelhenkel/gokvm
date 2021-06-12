package ssh

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	log "github.com/sirupsen/logrus"
)

func retrySshDial(attempts int, sleep time.Duration, f func(string, string, *ssh.ClientConfig) (*ssh.Client, error), network string, addr string, config *ssh.ClientConfig) error {
	if _, err := f(network, addr, config); err != nil {
		if err.Error() != "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain" ||
			err.Error() != fmt.Sprintf("dial tcp %s:22: connect: connection refused", addr) {
			if s, ok := err.(stop); ok {
				return s.error
			}

			if attempts--; attempts > 0 {
				// Add some randomness to prevent creating a Thundering Herd
				//log.Infof("waiting for ssh connection, retry attempt %d\n", attempts)
				//log.Infof("err %s\n", err)
				//jitter := time.Duration(rand.Int63n(int64(sleep)))
				//sleep = sleep + jitter/2
				//log.Infof("sleeping for %f\n", sleep.Seconds())
				time.Sleep(sleep)
				return retrySshDial(attempts, sleep, f, network, addr, config)
			}
			return err
		}
	}

	return nil
}

type stop struct {
	error
}

var Ch chan map[string]string = make(chan map[string]string)

func KeyScanCallback(hostname string, remote net.Addr, key ssh.PublicKey) error {
	Ch <- map[string]string{hostname[:len(hostname)-3]: string(ssh.MarshalAuthorizedKey(key))}
	return nil
}

func dial(server string, config *ssh.ClientConfig, wg *sync.WaitGroup) {
	if err := retrySshDial(60, time.Duration(time.Second*2), ssh.Dial, "tcp", fmt.Sprintf("%s:%d", server, 22), config); err != nil {
		log.Fatalln("Failed to dial:", err)
	}
	wg.Done()
}

var knownHostEntry string

func out(wg *sync.WaitGroup) {
	for s := range Ch {
		for k, v := range s {
			publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(v))
			if err != nil {
				log.Error(err)
				wg.Done()
			}
			khFilePath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
			f, err := os.OpenFile(khFilePath, os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				log.Error(err)
				wg.Done()
			}
			defer f.Close()
			knownHosts := knownhosts.Normalize(k)
			_, err = f.WriteString(knownhosts.Line([]string{knownHosts}, publicKey) + "\n")
			wg.Done()
		}

	}
}

func SSHKeyScan(username, host string) error {
	auth_socket := os.Getenv("SSH_AUTH_SOCK")
	if auth_socket == "" {
		return errors.New("no $SSH_AUTH_SOCK defined")
	}
	conn, err := net.DialTimeout("unix", auth_socket, time.Duration(time.Minute*1))
	if err != nil {
		return err
	}
	defer conn.Close()
	ag := agent.NewClient(conn)
	auths := []ssh.AuthMethod{ssh.PublicKeysCallback(ag.Signers)}

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            auths,
		HostKeyCallback: KeyScanCallback,
		Timeout:         time.Minute * 1,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go out(&wg)
	go dial(host, config, &wg)
	wg.Wait()
	return nil
}
