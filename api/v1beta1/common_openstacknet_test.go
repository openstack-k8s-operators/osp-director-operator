package v1beta1

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
)

func TestAddOSNetNameLowerLabels(t *testing.T) {

	tests := []struct {
		name                  string
		log                   logr.Logger
		labels                map[string]string
		networkNameLowerNames []string
		want                  map[string]string
	}{
		{
			name:                  "empty ip list",
			log:                   klogr.New(),
			labels:                map[string]string{},
			networkNameLowerNames: []string{},
			want:                  map[string]string{},
		},
		{
			name:                  "empty add network label",
			log:                   klogr.New(),
			labels:                map[string]string{},
			networkNameLowerNames: []string{"add"},
			want: map[string]string{
				"ooo-subnetname/add": "true",
			},
		},
		{
			name:                  "add network to existing label",
			log:                   klogr.New(),
			labels:                map[string]string{"foo": "bar"},
			networkNameLowerNames: []string{"add"},
			want: map[string]string{
				"foo":                "bar",
				"ooo-subnetname/add": "true",
			},
		},
		{
			name: "remove network label",
			log:  klogr.New(),
			labels: map[string]string{
				"foo":                   "bar",
				"ooo-subnetname/remove": "true",
			},
			networkNameLowerNames: []string{"add"},
			want: map[string]string{
				"foo":                "bar",
				"ooo-subnetname/add": "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			sortedIPs := AddOSNetNameLowerLabels(tt.log, tt.labels, tt.networkNameLowerNames)
			g.Expect(sortedIPs).NotTo(BeNil())
			g.Expect(sortedIPs).To(HaveLen(len(tt.want)))
			g.Expect(sortedIPs).To(BeEquivalentTo(tt.want))
		})
	}
}
