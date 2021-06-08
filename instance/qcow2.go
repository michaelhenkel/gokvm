package instance

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/michaelhenkel/gokvm/image"
	//qcow2 "github.com/zchee/go-qcow2"
)

type qemuImage struct {
	VirtualSize         int    `json:"virtual-size"`
	ActualSize          int    `json:"actual-size"`
	FullBackingFilename string `json:"full-backing-filename"`
}

func (i *Instance) createInstanceImage(backingImage image.Image) (*image.Image, error) {

	i.Image.Instance = i.Name
	i.Image.Name = "disk"
	i.Image.ImageType = image.INSTANCE
	found, err := i.Image.Get()
	if err != nil {
		return nil, err
	}
	if found {
		return &i.Image, nil
	}

	out, err := ioutil.TempDir("/tmp", "prefix")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(out)

	cmd := exec.Command("qemu-img", "create", "-b", i.Image.LibvirtImagePath+"/"+backingImage.Distribution+"/"+backingImage.Name, "-f", "qcow2", "-F", "qcow2", fmt.Sprintf("%s/%s", out, i.Name), i.Resources.Disk)
	_, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	i.Image.Name = "disk"
	i.Image.Instance = i.Name
	i.Image.ImageType = image.INSTANCE
	i.Image.ImageLocationType = image.File
	i.Image.ImageLocation = fmt.Sprintf("%s/%s", out, i.Name)
	i.Image.ImageType = image.INSTANCE
	if err := i.Image.Create(); err != nil {
		return nil, err
	}

	return &i.Image, nil
}
