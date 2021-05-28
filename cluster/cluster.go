package cluster

import (
	"github.com/michaelhenkel/gokvm/network"
)

type Cluster struct {
	Name    string
	Network network.Network
}
