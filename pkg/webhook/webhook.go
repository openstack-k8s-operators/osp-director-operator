// Can't put this in pkg/common because it creates an import loop with bindatautil

package webhook

import (
	"context"
	"path/filepath"

	bindatautil "github.com/openstack-k8s-operators/osp-director-operator/pkg/bindata_util"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureValidationWebhooks - Creates (if necessary) validation webhooks for "kind"
func EnsureValidationWebhooks(goClient client.Client, kind string) error {
	data := &bindatautil.RenderData{}
	data.Data = map[string]interface{}{}

	data.Data["Kind"] = kind

	// Generate the webhook objects
	objs, err := bindatautil.RenderTemplate(filepath.Join(bindatautil.ManifestPath, "webhook", "01-validating-webhook-configuration.yaml"), data)

	if err != nil {
		return err
	}

	// Apply the objects to the cluster
	for _, obj := range objs {
		obj.SetLabels(labels.Merge(obj.GetLabels(), map[string]string{
			"osp-director.openstack.org/webhook": "true",
		}))

		if err := bindatautil.ApplyObject(context.TODO(), goClient, obj); err != nil {
			return err
		}
	}

	return nil
}
