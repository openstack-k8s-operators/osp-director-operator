package common

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	corev1 "k8s.io/api/core/v1"

	"github.com/Masterminds/sprig"
)

// TType - TemplateType
type TType string

const (
	// TemplateTypeScripts - config type
	TemplateTypeScripts TType = "bin"
	// TemplateTypeConfig - config type
	TemplateTypeConfig TType = "config"
	// TemplateTypeCustom - custom config type
	TemplateTypeCustom TType = "custom"
	// TemplateTypeNone - none type, don't add configs from a directory, only files from AdditionalData
	TemplateTypeNone TType = "none"
)

// Template - config map and secret details
type Template struct {
	Name               string
	Namespace          string
	Type               TType
	InstanceType       string
	SecretType         corev1.SecretType // Secrets only, defaults to "Opaque"
	AdditionalTemplate map[string]string
	CustomData         map[string]string
	Labels             map[string]string
	Annotations        map[string]string
	ConfigOptions      map[string]interface{}
	SkipSetOwner       bool // skip setting ownership on the associated configmap
	Version            shared.OSPVersion
}

// GetTemplatesPath get path to templates, either running local or deployed as container
func GetTemplatesPath() string {

	templates := os.Getenv("OPERATOR_TEMPLATES")
	templatesPath := ""
	if templates == "" {
		// support local testing with 'up local'
		_, basefile, _, _ := runtime.Caller(1)
		templatesPath = path.Join(path.Dir(basefile), "../../templates")
	} else {
		// deployed as a container
		templatesPath = templates
	}

	return templatesPath
}

// GetAllTemplates get all files from a templates sub folder
func GetAllTemplates(path string, kind string, templateType string, version string) []string {

	templatePath := fmt.Sprintf("%s/%s/%s/*", path, strings.ToLower(kind), templateType)

	if version != "" {
		templatePath = fmt.Sprintf("%s/%s/%s/%s/*", path, strings.ToLower(kind), templateType, version)
	}

	templatesFiles, err := filepath.Glob(templatePath)
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}

	// remove any subdiretories from templatesFiles
	for index, file := range templatesFiles {
		fi, err := os.Stat(file)
		if err != nil {
			fmt.Print(err)
			os.Exit(1)
		}
		if fi.Mode().IsDir() {
			RemoveIndex(templatesFiles, index)
		}
	}

	return templatesFiles
}

// ExecuteTemplate creates a template from the file and
// execute it with the specified data
func ExecuteTemplate(templateFile string, data interface{}) (string, error) {

	b, err := os.ReadFile(templateFile)
	if err != nil {
		return "", err
	}
	file := string(b)

	renderedTemplate, err := ExecuteTemplateData(file, data)
	if err != nil {
		return "", err
	}
	return renderedTemplate, nil
}

// ExecuteTemplateData creates a template from string and
// execute it with the specified data
func ExecuteTemplateData(templateData string, data interface{}) (string, error) {

	var buff bytes.Buffer
	tmpl, err := template.New("tmp").Funcs(sprig.TxtFuncMap()).Parse(templateData)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&buff, data)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

// ExecuteTemplateFile creates a template from the file and
// execute it with the specified data
// Note: mschuppert - can be removed when all operators switched
//
//	to the above ones.
func ExecuteTemplateFile(filename string, data interface{}) (string, error) {

	templates := os.Getenv("OPERATOR_TEMPLATES")
	filepath := ""
	if templates == "" {
		// support local testing with 'up local'
		_, basefile, _, _ := runtime.Caller(1)
		filepath = path.Join(path.Dir(basefile), "../../templates/"+filename)
	} else {
		// deployed as a container
		filepath = path.Join(templates + filename)
	}

	b, err := os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	file := string(b)
	var buff bytes.Buffer
	tmpl, err := template.New("tmp").Funcs(sprig.TxtFuncMap()).Parse(file)
	if err != nil {
		return "", err
	}
	err = tmpl.Execute(&buff, data)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
}

// GetTemplateData -
func GetTemplateData(t Template) (map[string]string, error) {
	opts := t.ConfigOptions

	// get templates base path, either running local or deployed as container
	templatesPath := GetTemplatesPath()

	data := make(map[string]string)

	if t.Type != TemplateTypeNone {
		// get all scripts templates which are in ../templesPath/cr.Kind/CMType/<OSPVersion - optional>
		templatesFiles := GetAllTemplates(templatesPath, t.InstanceType, string(t.Type), string(t.Version))

		// render all template files
		for _, file := range templatesFiles {
			renderedData, err := ExecuteTemplate(file, opts)
			if err != nil {
				return data, err
			}
			data[filepath.Base(file)] = renderedData
		}
	}
	// add additional template files from different directory, which
	// e.g. can be common to multiple controllers
	for filename, file := range t.AdditionalTemplate {
		renderedTemplate, err := ExecuteTemplateFile(file, opts)
		if err != nil {
			return nil, err
		}
		data[filename] = renderedTemplate
	}

	return data, nil
}
