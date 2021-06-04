package cluster

import (
	"fmt"

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
}

func (c *Cluster) Delete() error {

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
	networkExists, err := c.Network.Get()
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

	imageExists, err := c.Image.Get()
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

	for i := 0; i < c.Controller; i++ {
		inst := instance.Instance{
			Name:        fmt.Sprintf("%s_%d", c.Name, i),
			PubKey:      c.PublicKey,
			Network:     c.Network,
			Image:       c.Image,
			ClusterName: c.Name,
			Suffix:      c.Suffix,
		}
		if err := inst.Create(); err != nil {
			return err
		}
	}

	return nil
}
