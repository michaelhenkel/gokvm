package cluster

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/instance"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/ssh"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"

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

	var wg sync.WaitGroup
	total := 4
	p := mpb.New(mpb.WithWaitGroup(&wg), mpb.WithWidth(32))
	for i := 0; i < c.Controller; i++ {
		name := fmt.Sprintf("c-instance-%d.%s.%s", i, c.Name, c.Suffix)
		bar := p.AddBar(int64(total),
			mpb.PrependDecorators(
				// simple name decorator
				decor.Name(name),
				decor.OnComplete(
					// spinner decorator with default style
					decor.Spinner(nil, decor.WCSyncSpace), "done",
				),
			),
			mpb.AppendDecorators(
				// decor.DSyncWidth bit enables column width synchronization
				decor.Percentage(decor.WCSyncWidth),
			),
		)

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
		wg.Add(1)
		go c.createInstance(inst, &wg, bar)
	}
	for i := 0; i < c.Worker; i++ {
		name := fmt.Sprintf("w-instance-%d.%s.%s", i, c.Name, c.Suffix)
		bar := p.AddBar(int64(total),
			mpb.PrependDecorators(
				decor.Name(name),
				decor.OnComplete(
					decor.Spinner(nil, decor.WCSyncSpace), "done",
				),
			),
			mpb.AppendDecorators(
				decor.Percentage(decor.WCSyncWidth),
			),
		)

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
		wg.Add(1)
		go c.createInstance(inst, &wg, bar)

	}
	wg.Wait()

	return nil
}

func (c *Cluster) createInstance(inst instance.Instance, wg *sync.WaitGroup, bar *mpb.Bar) error {
	defer wg.Done()
	if err := inst.Create(bar); err != nil {
		return err
	}
	bar.Increment()
	if err := c.waitForSSH(bar, inst.Name); err != nil {
		return err
	}
	bar.Increment()
	return nil

}

func (c *Cluster) waitForSSH(bar *mpb.Bar, instName string) error {
	for {
		newInst, _ := instance.Get(instName, c.Name)
		if len(newInst.IPAddresses) > 0 {
			if err := ssh.SSHKeyScan("root", newInst.IPAddresses[0]); err != nil {
				return err
			}
			return nil
		} else {
			time.Sleep(time.Millisecond)
		}
	}

}
