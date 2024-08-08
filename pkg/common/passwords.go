/*
Copyright 2021 Red Hat

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

package common

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"

	"github.com/google/uuid"

	math_rand "math/rand"
	"time"

	k8s_rand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	minPasswordSize = 25

	// PasswordMaxDepth , max depth of nested maps inside the pwd hirarchy.
	// Used when recursive converting existing password secret data from
	// map[interface{}]interface{} into nested maps of type map[string]interface{}.
	PasswordMaxDepth = 32
)

// PasswordNames returns an array with all service passwords used by tripleo
func PasswordNames() []string {
	return []string{
		"AdminPassword",
		"AdminToken",
		"AodhPassword",
		"BarbicanPassword",
		"BarbicanSimpleCryptoKek",
		"CeilometerMeteringSecret",
		"CeilometerPassword",
		"CephClientKey",
		"CephClusterFSID",
		"CephManilaClientKey",
		"CephRgwKey",
		"CephGrafanaAdminPassword",
		"CephDashboardAdminPassword",
		"CinderPassword",
		"CongressPassword",
		"DesignatePassword",
		"DesignateRndcKey",
		"Ec2ApiPassword",
		"EtcdInitialClusterToken",
		"GlancePassword",
		"GnocchiPassword",
		"HAProxyStatsPassword",
		"HeatAuthEncryptionKey",
		"HeatPassword",
		"HeatStackDomainAdminPassword",
		"HorizonSecret",
		"IronicPassword",
		"LibvirtTLSPassword",
		"KeystoneCredential0",
		"KeystoneCredential1",
		"KeystoneFernetKey0",
		"KeystoneFernetKey1",
		"KeystoneFernetKeys",
		"ManilaPassword",
		"MistralPassword",
		"MysqlClustercheckPassword",
		"MysqlMariabackupPassword",
		"MysqlRootPassword",
		"NeutronMetadataProxySharedSecret",
		"NeutronPassword",
		"NovaPassword",
		"NovajoinPassword",
		"MigrationSshKey",
		"OctaviaServerCertsKeyPassphrase",
		"OctaviaCaKeyPassphrase",
		"OctaviaHeartbeatKey",
		"OctaviaPassword",
		"PacemakerRemoteAuthkey",
		"PankoPassword",
		"PcsdPassword",
		"PlacementPassword",
		"RpcPassword",
		"NotifyPassword",
		"RabbitCookie",
		"RabbitPassword",
		"RedisPassword",
		"SaharaPassword",
		"SnmpdReadonlyUserPassword",
		"SwiftHashSuffix",
		"SwiftPassword",
		"ZaqarPassword",
	}
}

// GeneratePasswordsMap - generate map from passwordNames()
func GeneratePasswordsMap() map[string]interface{} {

	passwordsMap := make(map[string]interface{})
	for _, k := range PasswordNames() {
		passwordsMap[k] = ""
	}

	return passwordsMap
}

// GeneratePassword - creates a password for a password type
func GeneratePassword(pType string) interface{} {
	switch pType {
	case "CephClientKey", "CephManilaClientKey", "CephRgwKey":
		return generateCephKey()
	case "CephClusterFSID":
		return uuid.New()
	case "PacemakerRemoteAuthkey":
		return k8s_rand.String(4096)
	case "SnmpdReadonlyUserPassword":
		return k8s_rand.String(24)
	case "KeystoneFernetKeys":
		fernetJSON, _ := generateFernetContent()
		return fernetJSON
	case "KeystoneCredential0", "KeystoneCredential1", "KeystoneFernetKey0", "KeystoneFernetKey1":
		return generateFernetKey()
	case "MigrationSshKey":
		migrationJSON, _ := generateSSHKeypair()
		return migrationJSON
	case "BarbicanSimpleCryptoKek":
		return generateFernetKey()
	case "MysqlRootPassword":
		return k8s_rand.String(10)
	case "RabbitCookie":
		return k8s_rand.String(20)
	case "PcsdPassword":
		return k8s_rand.String(16)
	case "HorizonSecret":
		return k8s_rand.String(10)
	case "HeatAuthEncryptionKey":
		return k8s_rand.String(32)
	case "OctaviaServerCertsKeyPassphrase":
		return k8s_rand.String(32)
	case "DesignateRndcKey":
		return generateDesignateHMAC()
	default:
		return k8s_rand.String(minPasswordSize)
	}
}

// GeneratePasswords creates the passwords for a tripleo deployment
func GeneratePasswords() map[string]interface{} {
	passwords := GeneratePasswordsMap()
	for p := range passwords {
		passwords[p] = GeneratePassword(p)
	}

	return passwords
}

func generateCephKey() string {
	// https://github.com/ceph/ceph-deploy/blob/master/ceph_deploy/new.py#L21
	key := make([]byte, 16)
	for i := 0; i < 16; i++ {
		key[i] = byte(math_rand.Intn(10))
	}

	header := make([]byte, 12)
	binary.LittleEndian.PutUint16(header[0:], 1)                         // le16 type: CEPH_CRYPTO_AES
	binary.LittleEndian.PutUint32(header[2:], uint32(time.Now().Unix())) // le32 created: seconds
	binary.LittleEndian.PutUint32(header[6:], 0)                         // le32 created: nanoseconds
	binary.LittleEndian.PutUint16(header[10:], uint16(len(key)))         // le16: len(key)

	return base64.StdEncoding.EncodeToString(append(header, key...))
}

func generateDesignateHMAC() string {
	key := make([]byte, 16)
	for i := 0; i < 16; i++ {
		key[i] = byte(math_rand.Intn(10))
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(key)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func generateFernetKey() string {
	data := make([]byte, 32)
	for i := 0; i < 32; i++ {
		data[i] = byte(math_rand.Intn(10))
	}
	return base64.StdEncoding.EncodeToString(data)
}

type fernetKeyContent struct {
	Content string `json:"content"`
}

type fernetKey struct {
	Key1 fernetKeyContent `json:"/etc/keystone/fernet-keys/0"`
	Key2 fernetKeyContent `json:"/etc/keystone/fernet-keys/1"`
}

func generateFernetContent() (string, error) {
	f := fernetKey{fernetKeyContent{generateFernetKey()}, fernetKeyContent{generateFernetKey()}}
	b, err := json.Marshal(f)
	if err != nil {
		return "", err
	}
	return string(b), err

}

type privateKey struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func generateSSHKeypair() (string, error) {

	privKey, err := GeneratePrivateKey(4096)
	if err != nil {
		return "", err
	}

	pubKey, err := GeneratePublicKey(&privKey.PublicKey)
	if err != nil {
		return "", err
	}

	privateKeyPem := EncodePrivateKeyToPEM(privKey)

	p := privateKey{privateKeyPem, pubKey}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), err
}
