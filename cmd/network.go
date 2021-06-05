package cmd

import (
	"errors"
	"net"

	"github.com/michaelhenkel/gokvm/network"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var (
	subnet      string
	gateway     string
	dnsServer   string
	dhcp        bool
	networkType string
)

func init() {
	cobra.OnInitialize(initNetworkConfig)
	createNetworkCmd.PersistentFlags().StringVarP(&subnet, "subnet", "s", "", "")
	createNetworkCmd.PersistentFlags().StringVarP(&gateway, "gateway", "g", "", "")
	createNetworkCmd.PersistentFlags().StringVarP(&dnsServer, "dnsserver", "d", "", "")
	createNetworkCmd.PersistentFlags().BoolVarP(&dhcp, "dhcp", "a", true, "")
	createNetworkCmd.PersistentFlags().StringVarP(&networkType, "type", "t", "bridge", "")
}

func initNetworkConfig() {

}

func createNetwork() error {
	if name == "" {
		log.Fatal("Name is required")
	}
	if err := checkSubnet(subnet); err != nil {
		log.Fatal(err)
	}
	if err := checkGateway(gateway, subnet); err != nil {
		log.Fatal(err)
	}
	if err := checkDNS(dnsServer, subnet); err != nil {
		log.Fatal(err)
	}
	if err := checkNetworkType(networkType); err != nil {
		log.Fatal(err)
	}
	_, snipnet, err := net.ParseCIDR(subnet)
	if err != nil {
		return err
	}
	var gatewayIP, dnsIP net.IP
	if gateway == "" {
		_, gatewayNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}
		gatewayIP = gatewayNet.IP
		inc(gatewayIP)
	} else {
		gatewayIP = net.ParseIP(gateway)
	}

	if dnsServer == "" {
		dnsIP = gatewayIP
	} else {
		dnsIP = net.ParseIP(dnsServer)
	}
	newNetwork := &network.Network{
		Name:      name,
		DHCP:      dhcp,
		Subnet:    snipnet,
		DNSServer: dnsIP,
		Gateway:   gatewayIP,
		Type:      network.NetworkType(networkType),
	}
	if err := newNetwork.Create(); err != nil {
		return err
	}
	return nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

var createNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "creates a network",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := createNetwork(); err != nil {
			panic(err)
		}
	},
}

var deleteNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "deletes a network",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := deleteNetwork(); err != nil {
			panic(err)
		}
	},
}

var listNetworkCmd = &cobra.Command{
	Use:   "network",
	Short: "lists networks",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := listNetwork(); err != nil {
			panic(err)
		}
	},
}

func listNetwork() error {
	networks, err := network.List()
	if err != nil {
		return err
	}
	network.Render(networks)
	return nil
}

func checkSubnet(subnet string) error {
	if subnet == "" {
		return errors.New("subnet must be specified")
	}
	_, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return err
	}
	return nil
}

func deleteNetwork() error {
	if name == "" {
		log.Fatal("Name is required")
	}
	newNetwork := &network.Network{
		Name: name,
	}
	if err := newNetwork.Delete(); err != nil {
		return err
	}
	return nil
}

func checkGateway(gateway string, subnet string) error {
	if gateway != "" {
		if net.ParseIP(gateway) == nil {
			return errors.New("invalid gateway ip")
		}
		_, ipnet, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}
		if !ipnet.Contains(net.ParseIP(gateway)) {
			return errors.New("gateway ip not part of subnet")
		}
	}
	return nil
}

func checkDNS(dns string, subnet string) error {
	if dns != "" {
		if net.ParseIP(dns) == nil {
			return errors.New("invalid dns ip")
		}
		_, ipnet, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}
		if !ipnet.Contains(net.ParseIP(dns)) {
			return errors.New("dns ip not part of subnet")
		}
	}
	return nil
}

func checkNetworkType(networkType string) error {
	if networkType != "" {
		if networkType != string(network.OVS) && networkType != string(network.BRIDGE) {
			return errors.New("invalid networkType")
		}
	}
	return nil
}
