package cluster

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/instance"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/ssh"

	log "github.com/sirupsen/logrus"
)

type Cluster struct {
	Name       string
	Network    network.Network
	Image      image.Image
	Suffix     string
	Worker     int
	Controller int
	PublicKey  string
	Resources  instance.Resources
	Instances  []*instance.Instance
}

func List() ([]*Cluster, error) {
	instances, err := instance.List("")
	if err != nil {
		return nil, err
	}
	var clusterMap = make(map[string][]*instance.Instance)
	for _, inst := range instances {
		clusterMap[inst.ClusterName] = append(clusterMap[inst.ClusterName], inst)
	}
	var clusterList []*Cluster
	for k, v := range clusterMap {
		cl := &Cluster{
			Name:      k,
			Instances: v,
			Suffix:    v[0].Suffix,
		}
		clusterList = append(clusterList, cl)
	}
	return clusterList, nil
}

func Render(clusters []*Cluster) {
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Cluster", "Instances", "IP"})
	for _, cluster := range clusters {
		for _, inst := range cluster.Instances {
			ip := "not allocated yet"
			for _, addr := range inst.IPAddresses {
				ip = addr
			}
			t.AppendRow(table.Row{cluster.Name, inst.Name, ip}, rowConfigAutoMerge)
		}

	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetStyle(table.StyleLight)
	t.Render()
}

func (c *Cluster) Delete() error {
	instances, err := instance.List(c.Name)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		if err := inst.Delete(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cluster) Create() error {
	instances, err := instance.List(c.Name)
	if err != nil {
		return err
	}
	if len(instances) > 0 {
		log.Info("Cluster already exists")
		return nil
	}
	found, err := c.Image.Get()
	if err != nil {
		return err
	}
	if !found {
		defaultImage := image.DefaultImage()
		if err := defaultImage.Create(); err != nil {
			return err
		}
		c.Image = defaultImage
	}

	networkExists, err := network.Get(c.Network.Name)
	if err != nil {
		return err
	}
	if networkExists == nil {
		defaultNetwork := network.DefaultNetwork()
		defaultNetwork.Name = c.Network.Name
		if err := defaultNetwork.Create(); err != nil {
			return err
		}
		c.Network = defaultNetwork
	} else {
		c.Network = *networkExists
	}

	for i := 0; i < c.Controller; i++ {
		inst := instance.Instance{
			Name:        fmt.Sprintf("c-instance-%d.%s.%s", i, c.Name, c.Suffix),
			PubKey:      c.PublicKey,
			Network:     c.Network,
			Image:       c.Image,
			ClusterName: c.Name,
			Suffix:      c.Suffix,
			Resources:   c.Resources,
			Role:        instance.Controller,
		}
		if err := inst.Create(); err != nil {
			return err
		}
	}
	for i := 0; i < c.Worker; i++ {
		inst := instance.Instance{
			Name:        fmt.Sprintf("w-instance-%d.%s.%s", i, c.Name, c.Suffix),
			PubKey:      c.PublicKey,
			Network:     c.Network,
			Image:       c.Image,
			ClusterName: c.Name,
			Suffix:      c.Suffix,
			Resources:   c.Resources,
			Role:        instance.Worker,
		}
		if err := inst.Create(); err != nil {
			return err
		}
	}
	if err := c.waitForAddress(); err != nil {
		return err
	}
	if err := c.waitForSSH(); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) waitForSSH() error {
	clusters, err := List()
	if err != nil {
		return err
	}
	var cl *Cluster
	for _, cluster := range clusters {
		if cluster.Name == c.Name {
			cl = cluster
		}
	}
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Waiting for instances ssh: "
	s.Start()
	done := make(chan error)
	foundIPCounter := 0
	for _, inst := range cl.Instances {
		go func(inst *instance.Instance) {
			newInst, _ := instance.Get(inst.Name, c.Name)
			if len(newInst.IPAddresses) > 0 {
				if err := ssh.SSHKeyScan("root", newInst.IPAddresses[0]); err != nil {
					done <- err
				}
				time.Sleep(time.Millisecond)
				foundIPCounter = foundIPCounter + 1
				if foundIPCounter == len(cl.Instances) {
					done <- nil
				}
			}
		}(inst)
	}
	err = <-done
	s.Stop()
	switch err {
	case nil:
		return nil
	default:
		return err
	}
}

func (c *Cluster) waitForAddress() error {
	clusters, err := List()
	if err != nil {
		return err
	}
	var cl *Cluster
	for _, cluster := range clusters {
		if cluster.Name == c.Name {
			cl = cluster
		}
	}
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Prefix = "Waiting for instances ip: "
	s.Start()
	done := make(chan struct{})
	foundIPCounter := 0
	for _, inst := range cl.Instances {
		go func(inst *instance.Instance) {
			for {
				inst, _ := instance.Get(inst.Name, c.Name)
				if len(inst.IPAddresses) > 0 {
					time.Sleep(time.Millisecond)
					foundIPCounter = foundIPCounter + 1
					if foundIPCounter == len(cl.Instances) {
						done <- struct{}{}
					}
				} else {
					time.Sleep(time.Second * 1)
				}
			}
		}(inst)
	}
	<-done
	s.Stop()
	return nil
}
