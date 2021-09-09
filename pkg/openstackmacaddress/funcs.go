package openstackmacaddress

import (
	"crypto/rand"
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
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

	return mac, nil
}

// getAllMACReservations - get all MAC reservations in format MAC -> hostname
func getAllMACReservations(macNodeStatus map[string]ospdirectorv1beta1.OpenStackMACNodeStatus) map[string]string {
	ret := make(map[string]string)
	for hostname, status := range macNodeStatus {
		for _, mac := range status.Reservations {
			ret[mac] = hostname
		}
	}
	return ret
}

// IsUniqMAC - check if the hostname is uniq or already present in the reservations
// TODO: check if all MAC addresses with prefix from mac got consumed
func IsUniqMAC(macNodeStatus map[string]ospdirectorv1beta1.OpenStackMACNodeStatus, mac string) bool {
	reservations := getAllMACReservations(macNodeStatus)
	if _, ok := reservations[mac]; !ok {
		return true
	}
	return false
}
