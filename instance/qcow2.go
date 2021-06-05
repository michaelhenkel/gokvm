package instance

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/michaelhenkel/gokvm/image"
	qcow2 "github.com/zchee/go-qcow2"
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

	opts := &qcow2.Opts{
		Filename:      fmt.Sprintf("%s/%s", out, i.Name),
		BackingFile:   i.Image.Path,
		BackingFormat: "qcow2",
		Fmt:           "qcow2",
		Size:          int64(i.Resources.Disk),
		ClusterSize:   1024,
	}
	_, err = qcow2.Create(opts)
	if err != nil {
		return nil, err
	}
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
