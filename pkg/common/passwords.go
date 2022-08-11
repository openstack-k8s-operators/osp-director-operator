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

const minPasswordSize = 25

func passwordNames() []string {
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

// GeneratePasswords creates the passwords for a tripleo deployment
func GeneratePasswords() map[string]interface{} {

	passwords := make(map[string]interface{})
	for _, p := range passwordNames() {
		if p == "CephClientKey" || p == "CephManilaClientKey" || p == "CephRgwKey" {
			passwords[p] = generateCephKey()
		} else if p == "CephClusterFSID" {
			passwords[p] = uuid.New()
		} else if p == "PacemakerRemoteAuthkey" {
			passwords[p] = k8s_rand.String(4096)
		} else if p == "SnmpdReadonlyUserPassword" {
			passwords[p] = k8s_rand.String(24)
		} else if p == "KeystoneFernetKeys" {
			fernetJSON, _ := generateFernetContent()
			passwords[p] = fernetJSON
		} else if p == "KeystoneCredential0" || p == "KeystoneCredential1" || p == "KeystoneFernetKey0" || p == "KeystoneFernetKey1" {
			passwords[p] = generateFernetKey()
		} else if p == "MigrationSshKey" {
			migrationJSON, _ := generateSSHKeypair()
			passwords[p] = migrationJSON
		} else if p == "BarbicanSimpleCryptoKek" {
			passwords[p] = generateFernetKey()
		} else if p == "MysqlRootPassword" {
			passwords[p] = k8s_rand.String(10)
		} else if p == "RabbitCookie" {
			passwords[p] = k8s_rand.String(20)
		} else if p == "PcsdPassword" {
			passwords[p] = k8s_rand.String(16)
		} else if p == "HorizonSecret" {
			passwords[p] = k8s_rand.String(10)
		} else if p == "HeatAuthEncryptionKey" {
			passwords[p] = k8s_rand.String(32)
		} else if p == "OctaviaServerCertsKeyPassphrase" {
			passwords[p] = k8s_rand.String(32)
		} else if p == "DesignateRndcKey" {
			passwords[p] = generateDesignateHMAC()
		} else {
			passwords[p] = k8s_rand.String(minPasswordSize)
		}
	}
	return passwords
}

func generateCephKey() string {
	math_rand.Seed(time.Now().UnixNano())

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
	math_rand.Seed(time.Now().UnixNano())
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
