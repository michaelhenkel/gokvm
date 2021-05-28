package instance

import (
	"github.com/michaelhenkel/gokvm/image"
)

type Instance struct {
	Name   string
	Image  image.Image
	CPU    int
	Memory int
	Disk   string
}
