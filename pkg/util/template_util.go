package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// GetTemplatesPath get path to templates, either running local or deployed as container
func GetTemplatesPath() string {

	templates := os.Getenv("OPERATOR_TEMPLATES")
	templatesPath := ""
	if templates == "" {
		// support local testing with 'up local'
		_, basefile, _, _ := runtime.Caller(1)
		templatesPath = path.Join(path.Dir(basefile), fmt.Sprintf("../templates"))
	} else {
		// deployed as a container
		templatesPath = templates
	}

	return templatesPath
}

// GetAllTemplates get all files from a templates sub folder
func GetAllTemplates(path string, kind string, templateType string) []string {

	templatesFiles, err := filepath.Glob(fmt.Sprintf("%s/%s/%s/*", path, strings.ToLower(kind), templateType))
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
	return templatesFiles
}

// ExecuteTemplate creates a template from the file and
// execute it with the specified data
func ExecuteTemplate(templateFile string, data interface{}) string {

	b, err := ioutil.ReadFile(templateFile)
	if err != nil {
		panic(err)
	}
	file := string(b)

	return ExecuteTemplateData(file, data)
}

// ExecuteTemplateData creates a template from string and
// execute it with the specified data
func ExecuteTemplateData(templateData string, data interface{}) string {

	var buff bytes.Buffer
	tmpl, err := template.New("tmp").Parse(templateData)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(&buff, data)
	if err != nil {
		panic(err)
	}
	return buff.String()
}

// ExecuteTemplateFile creates a template from the file and
// execute it with the specified data
// Note: mschuppert - can be removed when all operators switched
//       to the above ones.
func ExecuteTemplateFile(filename string, data interface{}) string {

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

	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}
	file := string(b)
	var buff bytes.Buffer
	tmpl, err := template.New("tmp").Parse(file)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(&buff, data)
	if err != nil {
		panic(err)
	}
	return buff.String()
}
