package v1beta1

// MergeStringMaps - merge two or more string->map maps
func MergeStringMaps(baseMap map[string]string, extraMaps ...map[string]string) map[string]string {
	InitMap(&baseMap)

	for _, extraMap := range extraMaps {
		for key, value := range extraMap {
			baseMap[key] = value
		}
	}

	return baseMap
}

// InitMap - Inititialise a map to an empty map if it is nil.
func InitMap(m *map[string]string) {
	if *m == nil {
		*m = make(map[string]string)
	}
}
