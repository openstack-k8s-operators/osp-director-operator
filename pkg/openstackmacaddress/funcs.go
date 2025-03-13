// Package openstackmacaddress provides utilities for OpenStack MAC address management.
package openstackmacaddress

import (
	"crypto/rand"
	"fmt"
	"net"
)

// CreateMACWithPrefix - create random mac address using 3 byte prefix
func CreateMACWithPrefix(prefix string) (string, error) {
	buf := make([]byte, 3)
	_, err := rand.Read(buf)
	if err != nil {
		fmt.Println("error:", err)
		return "", err
	}
	mac := fmt.Sprintf("%s:%02x:%02x:%02x", prefix, buf[0], buf[1], buf[2])

	if _, err := net.ParseMAC(mac); err != nil {
		return "", fmt.Errorf("%s is an invalid MAC address", mac)
	}

	return mac, nil
}
