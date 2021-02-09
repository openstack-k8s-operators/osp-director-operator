package common

import (
	"context"

	virtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetAllVirtualMachinesWithLabel - Return an array of strings containing VirtualMachine names
func GetAllVirtualMachinesWithLabel(r ReconcilerCommon, labelSelectorMap map[string]string, namespace string) (*virtv1.VirtualMachineList, error) {
	virtualMachineList := &virtv1.VirtualMachineList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelectorMap) > 0 {
		labels := client.MatchingLabels(labelSelectorMap)
		listOpts = append(listOpts, labels)
	}

	err := r.GetClient().List(context.TODO(), virtualMachineList, listOpts...)

	if err != nil {
		return nil, err
	}

	return virtualMachineList, nil
}
