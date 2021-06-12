package image

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/michaelhenkel/gokvm/qemu"

	log "github.com/sirupsen/logrus"
	libvirt "libvirt.org/libvirt-go"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type ImageLocationType string
type ImageType string

const (
	URL          ImageLocationType = "url"
	File         ImageLocationType = "file"
	DISTRIBUTION ImageType         = "distribution"
	INSTANCE     ImageType         = "instance"
)

type Image struct {
	Name              string
	ImageLocationType ImageLocationType
	ImageLocation     string
	File              string
	MD5Name           string
	LibvirtImagePath  string
	ImagePath         string
	Distribution      string
	Instance          string
	ImageType         ImageType
	BackingImage      *Image
}

func DefaultImage() Image {
	return Image{
		Name:              "ubuntu2004",
		ImageLocationType: URL,
		ImageLocation:     "https://cloud-images.ubuntu.com/releases/focal/release-20210315/ubuntu-20.04-server-cloudimg-amd64.img",
		LibvirtImagePath:  "/var/lib/libvirt/images",
		Distribution:      "ubuntu",
		ImageType:         DISTRIBUTION,
	}
}

func (i *Image) Get() (bool, error) {

	images, err := List(i.ImageType)
	if err != nil {
		return false, err
	}
	for _, img := range images {
		if img.Name == i.Name {
			if i.Distribution != "" && img.Distribution == i.Distribution {
				*i = *img
				return true, nil
			}
			if i.Instance != "" && img.Instance == i.Instance {
				*i = *img
				return true, nil
			}
		}
	}
	return false, nil
}

func List(imageType ImageType) ([]*Image, error) {
	l, err := qemu.Connnect()
	if err != nil {
		return nil, err
	}
	pools, err := l.ListAllStoragePools(0)
	if err != nil {
		return nil, err
	}
	var images []*Image
	for _, pool := range pools {
		poolName, err := pool.GetName()
		if err != nil {
			return nil, err
		}
		var poolNameString string
		poolNameList := strings.Split(poolName, ":")
		switch imageType {
		case DISTRIBUTION:
			if len(poolNameList) == 3 && poolNameList[0] == "gokvm" && poolNameList[1] == "distribution" {
				poolNameString = poolNameList[2]
			}
		case INSTANCE:
			if len(poolNameList) == 3 && poolNameList[0] == "gokvm" && poolNameList[1] == "instance" {
				poolNameString = poolNameList[2]
			}
		}
		if poolNameString != "" {
			vols, err := pool.ListAllStorageVolumes(0)
			if err != nil {
				return nil, err
			}
			for _, vol := range vols {
				img, err := volumeToImage(vol, poolNameString)
				if err != nil {
					return nil, err
				}
				images = append(images, img)
			}
		}
	}
	return images, nil
}

func volumeToImage(vol libvirt.StorageVol, distroName string) (*Image, error) {
	volXML, err := vol.GetXMLDesc(0)
	if err != nil {
		return nil, err
	}
	var xmlVol libvirtxml.StorageVolume
	if err := xmlVol.Unmarshal(volXML); err != nil {
		return nil, err
	}
	img := &Image{
		Name:             xmlVol.Name,
		ImagePath:        xmlVol.Key,
		LibvirtImagePath: filepath.Dir(filepath.Dir(filepath.Dir(xmlVol.Key))),
	}
	imageType := filepath.Base(filepath.Dir(filepath.Dir(xmlVol.Key)))
	switch imageType {
	case string(INSTANCE):
		img.ImageType = INSTANCE
		img.Instance = filepath.Base(filepath.Dir(xmlVol.Key))
	case string(DISTRIBUTION):
		img.ImageType = DISTRIBUTION
		img.Distribution = filepath.Base(filepath.Dir(xmlVol.Key))
	}
	return img, nil
}

func Render(images []*Image) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Distribution", "Image"})
	var tableRows []table.Row
	for _, img := range images {
		tableRows = append(tableRows, table.Row{img.Distribution, img.Name})
	}
	t.AppendRows(tableRows)
	t.SetStyle(table.StyleLight)
	t.Render()
}

func (i *Image) Delete() error {
	l, err := qemu.Connnect()
	if err != nil {
		return nil
	}
	var poolName string
	switch i.ImageType {
	case DISTRIBUTION:
		poolName = fmt.Sprintf("gokvm:distribution:%s", i.Distribution)
	case INSTANCE:
		poolName = fmt.Sprintf("gokvm:instance:%s", i.Instance)
	}
	pool, err := l.LookupStoragePoolByName(poolName)
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
	poolXML, err := pool.GetXMLDesc(0)
	if err != nil {
		return err
	}
	xmlPool := libvirtxml.StoragePool{}
	if err := xmlPool.Unmarshal(poolXML); err != nil {
		return err
	}
	if len(vols) == 0 {
		if err := pool.Destroy(); err != nil {
			return err
		}
		if err := pool.Undefine(); err != nil {
			return err
		}
		if err := os.RemoveAll(xmlPool.Target.Path); err != nil {
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
	var poolName string
	switch i.ImageType {
	case DISTRIBUTION:
		poolName = fmt.Sprintf("gokvm:distribution:%s", i.Distribution)
	case INSTANCE:
		poolName = fmt.Sprintf("gokvm:instance:%s", i.Instance)
	}
	pool, err := l.LookupStoragePoolByName(poolName)
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
	found, err := i.Get()
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("image not created")
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
		log.Infof("Downloading image from %s\n", i.ImageLocation)
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

func readNBytes(r *bufio.Reader, peek int64) ([]byte, error) {
	return ioutil.ReadAll(io.LimitReader(r, peek))
}

func (i *Image) createPool() error {
	l, err := qemu.Connnect()
	if err != nil {
		return err
	}
	var poolName string
	switch i.ImageType {
	case DISTRIBUTION:
		poolName = fmt.Sprintf("gokvm:distribution:%s", i.Distribution)
		i.ImagePath = fmt.Sprintf("%s/distribution/%s", i.LibvirtImagePath, i.Distribution)
		if _, err := os.Stat(fmt.Sprintf("%s/distribution", i.LibvirtImagePath)); os.IsNotExist(err) {
			if err := os.Mkdir(fmt.Sprintf("%s/distribution", i.LibvirtImagePath), 0755); err != nil {
				return err
			}
		}
	case INSTANCE:
		poolName = fmt.Sprintf("gokvm:instance:%s", i.Instance)
		i.ImagePath = fmt.Sprintf("%s/instance/%s", i.LibvirtImagePath, i.Instance)
		if _, err := os.Stat(fmt.Sprintf("%s/instance", i.LibvirtImagePath)); os.IsNotExist(err) {
			if err := os.Mkdir(fmt.Sprintf("%s/instance", i.LibvirtImagePath), 0755); err != nil {
				return err
			}
		}
	}
	_, err = l.LookupStoragePoolByName(poolName)
	if err == nil {
		return nil
	}
	if _, err := os.Stat(i.ImagePath); os.IsNotExist(err) {
		if err := os.Mkdir(i.ImagePath, 0755); err != nil {
			return err
		}
	}
	storagePool := &libvirtxml.StoragePool{
		Name: poolName,
		Type: "dir",
		Target: &libvirtxml.StoragePoolTarget{
			Path: i.ImagePath,
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
