package image

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/qemu"

	log "github.com/sirupsen/logrus"
	libvirt "libvirt.org/libvirt-go"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type ImageLocationType string

const (
	URL  ImageLocationType = "url"
	File ImageLocationType = "file"
)

type Image struct {
	Name              string
	ImageLocationType ImageLocationType
	ImageLocation     string
	File              string
	MD5Name           string
	Path              string
	Pool              string
}

func DefaultImage() Image {
	return Image{
		Name:              "gokvm-default",
		ImageLocationType: URL,
		ImageLocation:     "https://cloud-images.ubuntu.com/releases/focal/release-20210315/ubuntu-20.04-server-cloudimg-amd64.img",
		Path:              "/var/lib/libvirt/images",
		Pool:              "gokvm",
	}
}

func (i *Image) Get() (*Image, error) {
	images, err := i.List()
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		if img.Name == i.Name {
			return img, nil
		}
	}
	return nil, nil
}

func (i *Image) List() ([]*Image, error) {
	l, err := qemu.Connnect()
	if err != nil {
		return nil, err
	}
	if i.Pool == "" {
		i.Pool = "gokvm"
	}
	pool, err := l.LookupStoragePoolByName(i.Pool)
	if err != nil {
		return nil, err
	}
	vols, err := pool.ListAllStorageVolumes(0)
	if err != nil {
		return nil, err
	}
	var images []*Image
	for _, vol := range vols {
		img, err := volumeToImage(vol, i.Pool)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}

	return images, nil
}

func volumeToImage(vol libvirt.StorageVol, poolName string) (*Image, error) {
	volXML, err := vol.GetXMLDesc(0)
	if err != nil {
		return nil, err
	}
	var xmlVol libvirtxml.StorageVolume
	if err := xmlVol.Unmarshal(volXML); err != nil {
		return nil, err
	}
	return &Image{
		Name: xmlVol.Name,
		Path: xmlVol.Key,
		Pool: poolName,
	}, nil
}

func Render(images []*Image) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Pool", "Volume"})
	var tableRows []table.Row
	for _, img := range images {
		tableRows = append(tableRows, table.Row{img.Pool, img.Name})
	}
	t.AppendRows(tableRows)
	t.SetStyle(table.StyleColoredBlackOnBlueWhite)
	t.Render()
}

func (i *Image) Delete() error {
	l, err := qemu.Connnect()
	if err != nil {
		return nil
	}
	pool, err := l.LookupStoragePoolByName(i.Pool)
	if err != nil {
		return nil
	}
	vol, err := pool.LookupStorageVolByName(i.Name)
	if err != nil {
		lerr, ok := err.(libvirt.Error)
		if !ok {
			return err
		}
		if lerr.Code != libvirt.ERR_NO_STORAGE_VOL {
			return err
		}
	}
	if vol != nil {
		if err := vol.Delete(0); err != nil {
			return err
		}
	}
	vols, err := pool.ListStorageVolumes()
	if err != nil {
		return nil
	}
	if len(vols) == 0 {
		if err := pool.Destroy(); err != nil {
			return err
		}
		if err := pool.Undefine(); err != nil {
			return err
		}
	}
	return nil
}

func (i *Image) Create() error {
	if err := i.createPool(); err != nil {
		return err
	}
	l, err := qemu.Connnect()
	if err != nil {
		return nil
	}
	pool, err := l.LookupStoragePoolByName(i.Pool)
	if err != nil {
		return nil
	}
	_, err = pool.LookupStorageVolByName(i.Name)
	if err != nil {
		lerr, ok := err.(libvirt.Error)
		if !ok {
			return err
		}
		if lerr.Code == libvirt.ERR_NO_STORAGE_VOL {
			return i.createVolume(pool, l)
		}
	}
	return nil

}

func (i *Image) createVolume(pool *libvirt.StoragePool, l *libvirt.Connect) error {
	dir, err := ioutil.TempDir("/tmp", "prefix")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	filename := fmt.Sprintf("%s/%s", dir, i.Name)
	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	switch i.ImageLocationType {
	case URL:
		resp, err := http.Get(i.ImageLocation)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}
	case File:
		f, err := os.Open(i.ImageLocation)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, f)
		if err != nil {
			return err
		}
	}

	fi, err := os.Stat(filename)
	if err != nil {
		return err
	}
	size := fi.Size()
	vol := libvirtxml.StorageVolume{
		Name: i.Name,
		Type: "file",
		Capacity: &libvirtxml.StorageVolumeSize{
			Unit:  "bytes",
			Value: uint64(size),
		},
	}
	volXML, err := vol.Marshal()
	if err != nil {
		return err
	}

	lvol, err := pool.StorageVolCreateXML(volXML, 0)
	if err != nil {
		log.Error("error creating volume")
		return err
	}

	stream, err := l.NewStream(0)
	if err != nil {
		return nil
	}

	if err := lvol.Upload(stream, 0, uint64(size), 0); err != nil {
		log.Error("error uploading")
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	r := bufio.NewReader(f)

	if err := stream.SendAll(func(stream *libvirt.Stream, i int) ([]byte, error) {
		return readNBytes(r, int64(i))
	}); err != nil {
		return err
	}
	return nil
}

func uploadVolume() {

}

func readNBytes(r *bufio.Reader, peek int64) ([]byte, error) {
	return ioutil.ReadAll(io.LimitReader(r, peek))
}

func (i *Image) createPool() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	_, err = l.LookupStoragePoolByName(i.Pool)
	if err == nil {
		log.Info("Pool already exists")
		return nil
	}
	if _, err := os.Stat(fmt.Sprintf("%s/%s", i.Path, i.Pool)); os.IsNotExist(err) {
		if err := os.Mkdir(fmt.Sprintf("%s/%s", i.Path, i.Pool), 0755); err != nil {
			return err
		}
	}
	storagePool := &libvirtxml.StoragePool{
		Name: i.Pool,
		Type: "dir",
		Target: &libvirtxml.StoragePoolTarget{
			Path: fmt.Sprintf("%s/%s", i.Path, i.Pool),
		},
	}
	poolXML, err := storagePool.Marshal()
	if err != nil {
		return err
	}
	p, err := l.StoragePoolDefineXML(poolXML, 0)
	if err != nil {
		return err
	}
	if err := p.SetAutostart(true); err != nil {
		return err
	}
	if err := p.Create(1); err != nil {
		return err
	}

	return nil
}
