package controlplane

// GetFencingRoles - roles that normally require fencing
func GetFencingRoles() []string {
	// Can't create an array constant, so this is what we'll do instead
	return []string{
		"CellController",
		"ControllerAllNovaStandalone",
		"ControllerNoCeph",
		"ControllerNovaStandalone",
		"ControllerOpenstack",
		"ControllerSriov",
		"ControllerStorageDashboard",
		"ControllerStorageNfs",
		"Controller",
		"Database",
		"Messaging",
		"Telemetry",
	}
}
