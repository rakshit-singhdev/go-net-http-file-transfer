
package helpers

import "net"

func GetLocalIPs() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ips []string

	for _, i := range interfaces {
		if i.Flags&net.FlagUp == 0 ||
			i.Flags&net.FlagLoopback != 0 ||
			i.Flags&net.FlagPointToPoint != 0 {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			ip := ipNet.IP.To4()

			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	return ips
}
