package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

type File struct {
	Filename    string
	ContentType string
	Content     string
}

var Files map[string]File = map[string]File{
	"MGH": File{Filename: "ass"},
}
var pattern string = `
    "%v" : File{
        Filename: "%v",
        ContentType: "%v",
        Content: ` + "`\n%v\n`,\n" + `
    },
`

func main() {
	if len(os.Args) == 1 {
		panic("No package name specified")
	}
	targetPackage := os.Args[1]
	if strings.EqualFold(targetPackage, "") {
		panic("No package name specified")
	}

	gopath := os.Getenv("GOPATH")
	if strings.EqualFold(gopath, "") {
		panic("GOPATH is empty")
	}

	rootDir := gopath + "/src/" + targetPackage
	configFile := rootDir + "/embed_go.json"
	f, e := os.Stat(rootDir)
	if e != nil {
		panic(e.Error())
	}

	if !f.IsDir() {
		panic(fmt.Sprintf("No directory exists in %v with name %v", gopath, targetPackage))
	}

	// load config file
	if _, e := os.Stat(configFile); e != nil {
		panic("Config file not found , Create config file with name embed_go.json in the root directory of your project")
	}

	var config []string
	filehandler, e := os.Open(configFile)
	jd := json.NewDecoder(filehandler)
	e = jd.Decode(&config)
	if e != nil {
		panic(e.Error())
	}
	tempPath := "./.tmp"
	tempFile, e := os.Create(tempPath)
	if e != nil {
		panic("Can not create temp file" + e.Error())
	}
	tempFile.Close()

	var initContent string = `
package embed_go

type File struct {
	Filename    string
	ContentType string
	Content     string
}
var Files map[string]File = map[string]File{`
	appendFile(tempPath, initContent)
	for _, item := range config {

		f, e := os.Stat(rootDir + "/" + item)
		target := rootDir + "/" + item
		if e != nil {
			panic(item + " not found")

		}

		if !f.IsDir() {
			//add file contents to file server
			bts, e := ioutil.ReadFile(target)
			if e != nil {
				continue
			}
			stripedFileName := stripRootDir(rootDir, target)
			mi := mime.TypeByExtension(filepath.Ext(target))
			str := fmt.Sprintf(pattern, stripedFileName, stripedFileName, mi, base64.StdEncoding.EncodeToString(bts))

			appendFile(tempPath, str)

		} else {
			filepath.Walk(target, func(path string, f os.FileInfo, err error) error {

				bts, e := ioutil.ReadFile(path)
				if e != nil {
					fmt.Println(e)
					return nil
				}
				stripedFileName := stripRootDir(rootDir, path)
				mi := mime.TypeByExtension(filepath.Ext(path))
				str := fmt.Sprintf(pattern, stripedFileName, stripedFileName, mi, base64.StdEncoding.EncodeToString(bts))

				e = appendFile(tempPath, str)
				if e != nil {
					log.Println("ERR", e.Error())
				}
				return nil
			})
		}
	}

	appendFile(tempPath, "\n}")

	e = os.Rename(tempPath, rootDir+"/embed_go/embed_go.go")
	if e != nil {
		return
	}
	createServerFile(rootDir)

}

func appendFile(filename string, text string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer f.Close()

	if _, err = f.WriteString(text); err != nil {
		return err
	}

	return nil
}
func stripRootDir(rootDir string, path string) string {
	return path[len(rootDir)+1:]
}

func createServerFile(rootDir string) {
	tempPath := ".server.tmp"
	tempFile, e := os.Create(tempPath)
	if e != nil {
		panic("Can not create temp file" + e.Error())
	}
	tempFile.WriteString(`
package embed_go

import (
	"encoding/base64"
	"net/http"
	"strings"
)

func ServeFiles(prefix string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Path

		if strings.EqualFold(r.URL.Path, "/") {
			filename = "/index.html"
		}
		file, ok := Files[prefix+filename]
		if ok == false {
			http.NotFound(w, r)
		}
		content, e := base64.StdEncoding.DecodeString(file.Content)
		if e != nil {
			http.NotFound(w, r)
		}
		w.Header().Add("Content-Type", file.ContentType)
		w.Write(content)

	}
}
`)
	tempFile.Close()
	os.Rename(tempPath, rootDir+"/embed_go/server.go")
}
