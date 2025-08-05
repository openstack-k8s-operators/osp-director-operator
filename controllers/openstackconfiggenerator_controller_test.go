/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	virtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	ospdirectorv1beta2 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta2"
	"github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
)

func TestCreateFencingEnvironmentFiles_RoleKeyVsRoleName(t *testing.T) {
	tests := []struct {
		name                 string
		roleKey              string            // Key in virtualMachineRoles map
		roleName             string            // Actual role name in roleName field
		vmLabels             map[string]string // Labels on the VM
		enableFencing        bool
		roleCount            int
		expectFencingConfigs bool // Whether we expect fencing configs to be generated
		expectError          bool
	}{
		{
			name:                 "matching role key and name - should work",
			roleKey:              "P101-Ctl",
			roleName:             "P101-Ctl",
			vmLabels:             map[string]string{common.OwnerNameLabelSelector: "p101-ctl"},
			enableFencing:        true,
			roleCount:            3,
			expectFencingConfigs: true,
			expectError:          false,
		},
		{
			name:                 "role key differs from role name - demonstrates bug",
			roleKey:              "P109-Ctl",                                                   // Role key in map
			roleName:             "P101-Ctl",                                                   // Actual role name
			vmLabels:             map[string]string{common.OwnerNameLabelSelector: "p101-ctl"}, // VM labeled with actual role name
			enableFencing:        true,
			roleCount:            3,
			expectFencingConfigs: true, // Test Bug OSPRH-18545: where role key was used, instead of the roleName to find the VMs
			expectError:          false,
		},
		{
			name:                 "fencing disabled",
			roleKey:              "P101-Ctl",
			roleName:             "P101-Ctl",
			vmLabels:             map[string]string{common.OwnerNameLabelSelector: "p101-ctl"},
			enableFencing:        false,
			roleCount:            3,
			expectFencingConfigs: false,
			expectError:          false,
		},
		{
			name:                 "wrong role count",
			roleKey:              "P101-Ctl",
			roleName:             "P101-Ctl",
			vmLabels:             map[string]string{common.OwnerNameLabelSelector: "p101-ctl"},
			enableFencing:        true,
			roleCount:            1, // Not 3, so no fencing
			expectFencingConfigs: false,
			expectError:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Setup test objects
			ctx := context.TODO()
			testLogger := log.FromContext(ctx).WithName("TestCreateFencingEnvironmentFiles_RoleKeyVsRoleName")

			// Create the fake Kubernetes client with our test objects
			s := runtime.NewScheme()
			_ = ospdirectorv1beta1.AddToScheme(s)
			_ = ospdirectorv1beta2.AddToScheme(s)
			_ = virtv1.AddToScheme(s)
			_ = corev1.AddToScheme(s)

			// Create OpenStackConfigGenerator instance
			instance := &ospdirectorv1beta1.OpenStackConfigGenerator{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config-generator",
					Namespace: "test-namespace",
				},
				Spec: ospdirectorv1beta1.OpenStackConfigGeneratorSpec{
					EnableFencing: tt.enableFencing,
				},
			}

			// Create VirtualMachineInstance with the specified labels
			vmi := &virtv1.VirtualMachineInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm-0",
					Namespace: "test-namespace",
					Labels:    tt.vmLabels,
				},
				Status: virtv1.VirtualMachineInstanceStatus{
					Interfaces: []virtv1.VirtualMachineInstanceNetworkInterface{
						{MAC: "aa:bb:cc:dd:ee:ff"},
					},
				},
			}

			// Create OpenStackControlPlane with role configuration
			controlPlane := &ospdirectorv1beta2.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-controlplane",
					Namespace: "test-namespace",
				},
				Spec: ospdirectorv1beta2.OpenStackControlPlaneSpec{
					VirtualMachineRoles: map[string]ospdirectorv1beta2.OpenStackVirtualMachineRoleSpec{
						tt.roleKey: {
							RoleName:  tt.roleName,
							RoleCount: tt.roleCount,
						},
					},
				},
			}

			// Create condition for testing
			cond := &shared.Condition{
				Type:    shared.ConfigGeneratorCondTypeInitializing,
				Status:  corev1.ConditionTrue,
				Reason:  shared.ConfigGeneratorCondReasonCMNotFound,
				Message: "Test condition",
			}

			// Setup fake client with test objects
			objs := []client.Object{instance, vmi, controlPlane}
			fakeClient := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()

			// Setup logger for debug output
			logger := zap.New(zap.UseDevMode(true))
			log.SetLogger(logger)

			// Create reconciler
			reconciler := &OpenStackConfigGeneratorReconciler{
				Client: fakeClient,
				Log:    logger,
				Scheme: s,
			}

			// Test the fencing environment files creation
			// Note: We need to create a custom tarball CM with fencing role for this to work
			tripleoTarballCM := createTarballConfigMapWithCustomRole(t, tt.roleName)
			cmLabels := make(map[string]string)

			fencingTemplate, err := reconciler.createFencingEnvironmentFiles(
				ctx,
				instance,
				cond,
				controlPlane,
				tripleoTarballCM,
				cmLabels,
			)
			if err != nil {
				testLogger.Info(err.Error())
			}

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(fencingTemplate).NotTo(BeNil())

			// Check if fencing is enabled in the template
			fencingYaml, exists := fencingTemplate["fencing.yaml"]
			g.Expect(exists).To(BeTrue())

			if tt.enableFencing && tt.roleCount > 1 && tt.expectFencingConfigs {
				// Should contain fencing device configuration
				g.Expect(fencingYaml).To(ContainSubstring("EnableFencing: true"))
				g.Expect(fencingYaml).To(ContainSubstring("fence_kubevirt"))
				g.Expect(fencingYaml).To(ContainSubstring("aa:bb:cc:dd:ee:ff")) // MAC address
				g.Expect(fencingYaml).To(ContainSubstring("test-vm-0"))         // VM name
			} else {
				// Should not contain fencing device configuration (or fencing disabled)
				if tt.enableFencing && tt.roleCount > 1 {
					g.Expect(fencingYaml).To(ContainSubstring("EnableFencing: true"))
					// But no devices due to the bug or other conditions
					g.Expect(fencingYaml).NotTo(ContainSubstring("fence_kubevirt"))
				} else {
					g.Expect(fencingYaml).To(ContainSubstring("EnableFencing: false"))
				}
			}
		})
	}
}

// createTarballConfigMapWithCustomRole creates a ConfigMap with binary data containing a custom role
// that includes the Pacemaker service (required for fencing)
func createTarballConfigMapWithCustomRole(t *testing.T, roleName string) *corev1.ConfigMap {
	// This is a simplified version - in reality this would be a proper tarball
	// For the test, we just need to ensure GetCustomFencingRoles returns our role

	// In a real test, this would be a proper gzipped tarball containing roles_data.yaml
	// For simplicity, we'll use the base64 encoded data from the existing test
	// This data contains a role with Pacemaker service that will be detected as a fencing role
	customTarballData := "H4sIAAAAAAAAA+1bbW/bOBLO5/wKYoteukDTxmleDv6W2MnGaPyyUTZb3OFg0BRtE5FIgaTseLE//oaU5NSp26HdBLfX1QBpYukZajRvnOG4797vvDgdAJ0eH7jfjSe/K9ppHB8cfWgcnRwenO4cNBpHR4c75PjlRdvZyY2lmpCd1LBpnmX8azjs/v8pvXuvVcLNMKaWvlvQNHmBZzgDnxx9zf4fjg8PTnYaR6eNg9OTw2Nv/6Ojk8YOOXgBWb6gv7n9Xz0v7b4ilyLhZMIl19TymIwW5FaLLOH93Wd/1nOvR24gFppk0Dho7LdsQr6Lnl++fSJp+pl8u4TE3DDQrhVKNsmfu+65S+mFIUxJC/GdcE1cmBM7pZZMqSE0SeADrwBw33A9E4wbkigag92ojAEpY0gOftk2JIgRNfwt6XJj6ETIicf0uJ0rfU/GuWRODPMO4C2VS9vmY5ontkkacMXSiWn6hfZJpkVK9aL89Cijv/CKtDQHzyH8wXItaQJPyAEgyUiLeMLJWGkS9c5uyRv3+DGIa50snYEh8ymXJDfwsVyqe334vn8XkbmwU5Vb0r67+bl8bLX8sFgWrsriTUoxL8r7xSdCTD4CQPORr7jg73Zkce0sE0/horw1pJn4nCWyStMJfwo3xeXPkbdcUmmfAq2/+oiDuAO9mIWxPDX+fclIwT+dwezI2wn+OHlLFionKV0Qk3EmxgtCyQR0PYcrsrKj0n45Ttn0LTyOTQn4y7/3WoWZBgmVfO8t2av0s/cf74be1EMNSubDpSKBbQV2pYx1LgyipvTRP/ZeQw5m9+7O6/3MuS+zyf5rIWP+8HrPeYRKUy4tcSYUYyKVBXGZc0O9AA2Tm6t+NCCN03cNAn/Eff8CbZ5pznwO+gcZwfpzquN9ptIMHGYEwTCjSQ7+/uay86l70SRdes9JRjVIASL72DEC1Cntz369iFvwLbdVLhceerghVkGOy7mTjcoFeQSQEkA1d7yxi431azT9Cl6Vq3eG4HCagjBjMQFVVdF94a62/NW9dVzjhM6UBob+jGuWqDwuDXjpb6zlgagEj3x8Rsd9foKUgg0LYYZFMtp7jF9fPzh8nkE5AU7MtYAA8vEflemlMnoZhf2o2Sy3h2azwjSbZyLmCELFUwg3HHThzEyt0jj02pubByB7yoqxQJF5LGz725hzqkeCUYm+SwU8B0/mMo5ECoiWXmRWbcTYVhPIxRuxfExFthHD4J6ZRiNIOKXswAfAt3GtsxbXFgNxkSgfvWdQf9gW/GiabMRTGBZcHXYyjDGbthIBPDiuSoI48hdNx1RSHNiNUWUAaIJ4qAeFvGlvHPC4m1HcFVpjoeahkzkCctlfo1FRwCrX5kkSsQ0ZLrqtgZpzfZnwhy1ZU7otp6sCkES3jnebl/xNCrvYnO2ue/apE7WizhasvU9bMD04Q/SjzTk/Wc1TgeSbFUYoXc8yJLOtMAzyjawVMZrwTn8Tie66vH8ZypAHyX41uLjmY3sFRWCwISM25XGeYBtcAb5TSZ5iekmgz4ZCZMrZPYJ0pQSzMYaSlgrYq32NMtAcKhhEhDY3YiKhLkGTyhIZtIMs0QOt4pxhOlvCf4cSORjcbfeQkFhCIyERFS+h52C/b0MvLMMQ5d4WzcXYggoekCRzqZF3/gU6DIYbaQmrmi8ELhVjU4EvW+C63GqBvXqJjSy1BoFenWW4aq6g70UFLEEtV9LPqWXTMPgY2eQd7kJCV49E0ZXS4g+sYuhkNKQ06mSGIztZBxp/wVClFDBIChCAaKlfgDvS9cCB4MEDopYCVp5U+EoSwRtmBOIzH8GtOeLWHzl0/Aoz2rUCw0LnhdRwXSpFQlFlF7Byx3LF3GXQuiVDx0DBjfjPCgPs0hTbpVcYfGXVxWqyFZaA4mgFj5Y2q2iooULggbtuCZ6iO16Xp4y6NTGYy3Xm1xh7LvTtIqN2iq23iH69DoCEpIgqojCfLHHnk+wuC0ZHYzyzlNAW1OmDJIcwCoK3pywLyAIl+vpwMt8QHSj29YcN1hUyfygOQ8OZwHeoGx2Fc/RnJhx8l22gRo80nfPiVAxhUTM80TlQ4J7ioMUegOMqneHIwITgoHeSBRRfy7LZwJaglXU1LkiNBHSfWToTuL5KXJtniVq4A9sQS5RMUH8kdgqpDcp55HUrDpUbfs95Vh33YwwhBXc/4zJyh9FFckI2tv5dr32OY1qrE46vQgeU8ZSiMg4ShwPpcIOYRC2nNTcZ4psraH8QhnjTQMUpRRLiDY8FoqGbqUkRhFmAbIiVS1AkYs4oosKIM2jlLbbnRzJFCo/IYLthaF/kcW0BRak26OGjB9+Amc5zkcSYw3h0OXD6NvJWpNwsJOIoDvUHWnTCpUQoNzZM1SzgKK/A/56k4UgX0W5ciTjYEh+YT5d4lxtRKFxSl0LzOU2QYr0EQ5jfgyUwoXOJFW6/yZHKseQNW+jLDsqjGSPfRS88KAf5dtcOys+pEawaTLkZY5ZbTnoq5n5YvrtujP0Km06/yaXhfkrp59Bfm0B/Pg5nxZPLT9hceotZc9gEOXwmHTzGNTO2HOMCn3OawXLAujIJvDRnQkHP1svTEdeg64Ojfx6fnhTiu0iADDoWCe95u/40E9rmNNmfgiQ/7f6Vp72vyO2UG/7FsNUQJRM3dQc2WHBUcPgvN/AHEMT5jv9mlh/eu4l3NZh376QMde/kAdvMlKtJry9b10x5102eHbYKKGz6nJYl7hM2F2vQmpc313KaKkUPp+DO5ukC1d0rd3P9EDt7ytQZPEUaL8dQc6NyzXg1yf6cKWDs7aP2azPvw+PdFxh6hwyUn3ei+uxDzsDjfa/cJ8NZTIYVcCdLsdlh8ZANO/xVrqAGe5Vlg5Z5lTGsc0bP15/xcPaZDjFDjydf4pxqg0Oo4nDpnE4ykSGB+j1HKoFnHu4A41+hRx2tlRLjG8BrMXJ7azDwF9hTsSTizzzERPsvVdxSPeGo229zShHUjf7QreYLdm9/4S7oDhsObHACA9DV9PzcnclK51TWGpJsSy/eOcn1ndNS8C+apuKbp07j3ajveQxx/RA1BNoNqFn32ZQaqLT3qdn33wGqe6zv7bFk3WP9gD2W3K7Hkt/bY8lteiy5TY8le9zWbVbdZtVtVt1m1W1W3WbVbdbfts36X/8v2JpqqqmmmmqqqaaaaqqppppqqqmmH5f+C9VfCEoAUAAA"

	data, err := base64.StdEncoding.DecodeString(string(customTarballData))
	if err != nil {
		return nil
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tripleo-tarball-config",
			Namespace: "test-namespace",
		},
		BinaryData: map[string][]byte{
			"tripleo-tarball-config.tar.gz": []byte(data),
		},
	}
}
