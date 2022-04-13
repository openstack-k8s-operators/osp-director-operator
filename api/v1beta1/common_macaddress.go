package v1beta1

// Because webhooks *MUST* exist within the api/<version> package, we need to place common
// functions here that might be used across different Kinds' webhooks

// getAllMACReservations - get all MAC reservations in format MAC -> hostname
func getAllMACReservations(macNodeStatus map[string]OpenStackMACNodeReservation) map[string]string {
	ret := make(map[string]string)
	for hostname, status := range macNodeStatus {
		for _, mac := range status.Reservations {
			ret[mac] = hostname
		}
	}
	return ret
}

// IsUniqMAC - check if the MAC address is uniq or already present in the reservations
func IsUniqMAC(macNodeStatus map[string]OpenStackMACNodeReservation, mac string) bool {
	reservations := getAllMACReservations(macNodeStatus)
	if _, ok := reservations[mac]; !ok {
		return true
	}
	return false
}
