package image

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/michaelhenkel/gokvm/qemu"

	log "github.com/sirupsen/logrus"
	libvirt "libvirt.org/libvirt-go"
	libvirtxml "libvirt.org/libvirt-go-xml"
)

type Image struct {
	Name    string
	URL     string
	MD5Name string
	Path    string
	Pool    string
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
	log.Info(i.URL, filename)
	resp, err := http.Get(i.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	fi, err := os.Stat(filename)
	if err != nil {
		return err
	}

	// get the size
	size := fi.Size()
	log.Info("size ", size)
	log.Info(filename)
	vol := libvirtxml.StorageVolume{
		Name: i.Name,
		Type: "file",
		Capacity: &libvirtxml.StorageVolumeSize{
			Unit:  "bytes",
			Value: 2361393152,
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

	if err := lvol.Upload(stream, 0, 2361393152, 0); err != nil {
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
