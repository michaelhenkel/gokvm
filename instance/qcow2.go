package instance

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/michaelhenkel/gokvm/image"

	log "github.com/sirupsen/logrus"
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
		log.Error("cannot get image", err)
		return nil, err
	}
	if found {
		return &i.Image, nil
	}

	out, err := ioutil.TempDir("/tmp", "prefix")
	if err != nil {
		log.Error("cannot create temp dir", err)
		return nil, err
	}
	defer os.RemoveAll(out)
	backingImgPath := fmt.Sprintf("%s/distribution/%s/%s", i.Image.LibvirtImagePath, backingImage.Distribution, backingImage.Name)
	imgOut := fmt.Sprintf("%s/%s", out, i.Name)
	cmdLine := []string{"create", "-b", backingImgPath, "-f", "qcow2", "-F", "qcow2", imgOut, i.Resources.Disk}

	cmd := exec.Command("qemu-img", cmdLine...)
	_, err = cmd.Output()
	if err != nil {
		log.Error("cannot create cloud init img", err)
		return nil, err
	}

	i.Image.Name = "disk"
	i.Image.Instance = i.Name
	i.Image.ImageType = image.INSTANCE
	i.Image.ImageLocationType = image.File
	i.Image.ImageLocation = fmt.Sprintf("%s/%s", out, i.Name)
	i.Image.ImageType = image.INSTANCE
	if err := i.Image.Create(); err != nil {
		log.Error("cannot create image volume", err)
		return nil, err
	}

	return &i.Image, nil
}
