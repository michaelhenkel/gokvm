package instance

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/michaelhenkel/gokvm/image"
	//qcow2 "github.com/zchee/go-qcow2"
)

func (i *Instance) createInstanceImage() (*image.Image, error) {
	existingImg, err := image.Get(i.Name, i.Image.Pool)
	if err != nil {
		return nil, err
	}
	if existingImg != nil {
		return existingImg, nil
	}

	out, err := ioutil.TempDir("/tmp", "prefix")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(out)
	/*
		log.Info("disk size", int64(i.Resources.Disk))

		opts := &qcow2.Opts{
			Filename:      fmt.Sprintf("%s/%s", out, i.Name),
			BackingFile:   i.Image.Path,
			BackingFormat: "qcow2",
			Fmt:           "qcow2",
			Size:          int64(i.Resources.Disk),
			ClusterSize:   512,
		}
		_, err = qcow2.Create(opts)
		if err != nil {
			return nil, err
		}
	*/
	cmd := exec.Command("qemu-img", "create", "-b", i.Image.Path, "-f", "qcow2", "-F", "qcow2", fmt.Sprintf("%s/%s", out, i.Name), i.Resources.Disk)
	_, err = cmd.Output()
	if err != nil {
		return nil, err
	}
	//log.Info(string(stdout))
	//qemu-img create -b ${imageName} -f qcow2 -F qcow2 ${libvirtImageLocation}/${imageName}-${clusterName}-${hostname}.qcow2 ${disk}
	img := &image.Image{
		Pool:              i.Image.Pool,
		Name:              i.Name,
		ImageLocationType: image.File,
		ImageLocation:     fmt.Sprintf("%s/%s", out, i.Name),
	}
	if err := img.Create(); err != nil {
		return nil, err
	}
	img, err = image.Get(img.Name, img.Pool)
	if err != nil {
		return nil, err
	}

	return img, nil
}
