package openstacknetattachment

import (
	"context"
	"fmt"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"

	//openstacknet "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknet"
	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOpenStackNetAttachmentWithLabel - Return OpenStackNet with labels
func GetOpenStackNetAttachmentWithLabel(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	labelSelector map[string]string,
) (*ospdirectorv1beta1.OpenStackNetAttachment, error) {
	osNetAttachList, err := GetOpenStackNetAttachmentsWithLabel(
		ctx,
		r,
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}
	if len(osNetAttachList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(v1.Resource("openstacknetattachment"), fmt.Sprint(labelSelector))
	} else if len(osNetAttachList.Items) > 1 {
		return nil, fmt.Errorf("multiple OpenStackNetAttachments with label %v not found", labelSelector)
	}
	return &osNetAttachList.Items[0], nil
}

// GetOpenStackNetAttachmentsWithLabel - Return a list of all OpenStackNetAttachmentss in the namespace that have (optional) labels
func GetOpenStackNetAttachmentsWithLabel(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	labelSelector map[string]string,
) (*ospdirectorv1beta1.OpenStackNetAttachmentList, error) {
	osNetAttachList := &ospdirectorv1beta1.OpenStackNetAttachmentList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := r.GetClient().List(ctx, osNetAttachList, listOpts...); err != nil {
		return nil, err
	}

	return osNetAttachList, nil
}

// GetOpenStackNetAttachmentWithAttachReference - Return OpenStackNetAttachment for the reference name use in the osnet config
func GetOpenStackNetAttachmentWithAttachReference(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	attachReference string,
) (*ospdirectorv1beta1.OpenStackNetAttachment, error) {
	osNetAttach, err := GetOpenStackNetAttachmentWithLabel(
		ctx,
		r,
		namespace,
		map[string]string{
			AttachReference: attachReference,
		},
	)
	if err != nil {
		return nil, err
	}

	return osNetAttach, nil
}

// GetOpenStackNetAttachmentType - Return type of OpenStackNetAttachment, either bridge or sriov
func GetOpenStackNetAttachmentType(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	attachReference string,
) (*ospdirectorv1beta1.AttachType, error) {

	osNetAttach, err := GetOpenStackNetAttachmentWithAttachReference(
		ctx,
		r,
		namespace,
		attachReference,
	)
	if err != nil {
		return nil, err
	}

	return &osNetAttach.Status.AttachType, nil
}

// GetOpenStackNetAttachmentBridgeName - Return name of the Bridge configured by the OpenStackNetAttachment
func GetOpenStackNetAttachmentBridgeName(
	ctx context.Context,
	r common.ReconcilerCommon,
	namespace string,
	attachReference string,
) (string, error) {

	osNetAttach, err := GetOpenStackNetAttachmentWithAttachReference(
		ctx,
		r,
		namespace,
		attachReference,
	)
	if err != nil {
		return "", err
	}

	return osNetAttach.Status.BridgeName, nil
}
