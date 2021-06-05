package cluster

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/instance"
	"github.com/michaelhenkel/gokvm/network"

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
		}
		clusterList = append(clusterList, cl)
	}
	return clusterList, nil
}

func Render(clusters []*Cluster) {
	rowConfigAutoMerge := table.RowConfig{AutoMerge: true}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Cluster", "Instances"})
	for _, cluster := range clusters {
		for _, inst := range cluster.Instances {
			t.AppendRow(table.Row{cluster.Name, inst.Name}, rowConfigAutoMerge)
		}

	}
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true},
	})
	t.SetStyle(table.StyleColoredBlackOnBlueWhite)
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

	imageExists, err := image.Get(c.Image.Name, c.Image.Pool)
	if err != nil {
		return err
	}
	if imageExists == nil {
		defaultImage := image.DefaultImage()
		defaultImage.Name = c.Image.Name
		if err := defaultImage.Create(); err != nil {
			return err
		}
		c.Image = defaultImage
	} else {
		c.Image = *imageExists
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
		}
		if err := inst.Create(); err != nil {
			return err
		}
	}

	return nil
}
