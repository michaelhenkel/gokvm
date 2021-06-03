package cmd

import (
	"github.com/spf13/cobra"

	"github.com/michaelhenkel/gokvm/image"

	log "github.com/sirupsen/logrus"
)

var (
	url          string
	md5url       string
	path         string
	pool         string
	locationType string
)

func init() {
	cobra.OnInitialize(initImageConfig)
	createImageCmd.PersistentFlags().StringVarP(&url, "url", "u", "", "")
	createImageCmd.PersistentFlags().StringVarP(&md5url, "md5url", "m", "", "")
	createImageCmd.PersistentFlags().StringVarP(&path, "path", "p", "", "")
	createImageCmd.PersistentFlags().StringVarP(&locationType, "locationtype", "l", "", "")
	createImageCmd.PersistentFlags().StringVarP(&pool, "pool", "s", "", "")
}

func initImageConfig() {

}

var createImageCmd = &cobra.Command{
	Use:   "image",
	Short: "creates an image",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := createImage(); err != nil {
			panic(err)
		}
	},
}

var deleteImageCmd = &cobra.Command{
	Use:   "image",
	Short: "deletes an image",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := deleteImage(); err != nil {
			panic(err)
		}
	},
}

var listImageCmd = &cobra.Command{
	Use:   "image",
	Short: "lists images",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := listImage(); err != nil {
			panic(err)
		}
	},
}

func createImage() error {
	if name == "" {
		log.Fatal("Name is required")
	}
	if pool == "" {
		pool = "gokvm"
	}
	if path == "" {
		path = "/var/lib/libvirt/images"
	}
	if url == "" {
		url = "https://cloud-images.ubuntu.com/releases/focal/release-20210315/ubuntu-20.04-server-cloudimg-amd64.img"
		url = "http://localhost:8000/ubuntu-20.04-server-cloudimg-amd64.img"
	}
	if locationType == "" {
		locationType = string(image.URL)
	}
	i := image.Image{
		Name:              name,
		Pool:              pool,
		Path:              path,
		ImageLocationType: image.ImageLocationType(locationType),
		ImageLocation:     url,
	}
	return i.Create()
}

func listImage() error {
	if pool == "" {
		pool = "gokvm"
	}
	i := image.Image{
		Pool: pool,
	}
	images, err := i.List()
	if err != nil {
		return err
	}
	image.Render(images)
	return nil
}

func deleteImage() error {
	if name == "" {
		log.Fatal("Name is required")
	}
	if pool == "" {
		pool = "gokvm"
	}
	i := image.Image{
		Name: name,
		Pool: pool,
	}
	return i.Delete()
}
