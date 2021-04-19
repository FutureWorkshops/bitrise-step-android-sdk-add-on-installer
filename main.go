package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/bitrise-io/go-steputils/stepconf"
	"github.com/bitrise-io/go-utils/log"
)

func fail(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

type Checksum struct {
	XMLName  xml.Name `xml:"checksum"`
	Type     string   `xml:"type,attr"`
	CheckSum string   `xml:",chardata"`
}

type Archive struct {
	XMLName  xml.Name `xml:"archive"`
	OS       string   `xml:"os,attr"`
	Size     string   `xml:"size"`
	Checksum Checksum `xml:"checksum"`
	Url      string   `xml:"url"`
}

type Extra struct {
	XMLName     xml.Name  `xml:"extra"`
	Vendor      string    `xml:"vendor"`
	Path        string    `xml:"path"`
	Description string    `xml:"description"`
	Revision    string    `xml:"revision"`
	ApiLevel    string    `xml:"min-api-level"`
	Url         string    `xml:"desc-url"`
	Archives    []Archive `xml:"archives>archive"`
}

type Addon struct {
	XMLName xml.Name `xml:"sdk-addon"`
	Extra   Extra    `xml:"extra"`
}

type Config struct {
	AddOnURL       string `env:"add_on_url"`
	AndroidSDKPath string `env:"android_sdk_path"`
	VerboseLog     bool   `env:"verbose_log,opt[yes,no]"`
}

func (cfg Config) validate() error {
	if cfg.AddOnURL == "" {
		return fmt.Errorf("no Add-on URL")
	}

	_, err := url.ParseRequestURI(cfg.AddOnURL)
	if err != nil {
		return fmt.Errorf("invalid Add-on URL")
	}

	return nil
}

func (cfg Config) downloadXml() ([]byte, error) {
	resp, err := http.Get(cfg.AddOnURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}

// DownloadFile will download a url and store it in local filepath.
// It writes to the destination file as it downloads it, without
// loading the entire file into memory.
func (archive Archive) downloadFile(root string) (string, error) {

	// Create the file
	out, err := ioutil.TempFile(os.TempDir(), archive.Checksum.CheckSum)
	if err != nil {
		return "", err
	}

	stat, err := out.Stat()
	if err != nil {
		return "", err
	}
	var archive_path = os.TempDir() + stat.Name()

	log.Debugf("Download archive into %s", archive_path)

	defer out.Close()

	// Get the data
	resp, err := http.Get(archive.Url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return archive_path, nil
}

func (cfg Config) prepareExtraFolder() (string, error) {
	var extra_path = cfg.AndroidSDKPath + "/extras"
	_, err := os.Stat(extra_path)

	if os.IsNotExist(err) {
		os.Mkdir(extra_path, 0755)
		log.Infof("Extra folder created at %s", extra_path)
	} else {
		log.Debugf("Extra folder already exists at %s", extra_path)
	}

	if _, err := os.Stat(extra_path); err != nil {
		return "", err
	}

	return extra_path, nil
}

func (extra Extra) createFolderStructure(root string) string {
	var vendor_folder = root + "/" + strings.ToLower(extra.Vendor)

	if _, err := os.Stat(vendor_folder); os.IsNotExist(err) {
		os.Mkdir(vendor_folder, 0755)
		log.Infof("Vendor folder created at %s", vendor_folder)
	} else {
		log.Debugf("Vendor folder already exists at %s", vendor_folder)
	}

	var sdk_folder = vendor_folder + "/" + strings.ToLower(extra.Path)
	if _, err := os.Stat(sdk_folder); os.IsNotExist(err) {
		os.Mkdir(sdk_folder, 0755)
		log.Infof("SDK folder created at %s", sdk_folder)
	} else {
		log.Debugf("SDK folder already exists at %s", sdk_folder)
	}

	return sdk_folder
}

func unzipContent(file string, destination string) error {
	commandString := fmt.Sprintf(`unzip -qd %s %s`, destination, file)
	log.Debugf("Unziping with command %s", commandString)

	commandSlice := strings.Fields(commandString)
	c := exec.Command(commandSlice[0], commandSlice[1:]...)
	e := c.Run()

	return e
}

func main() {
	var cfg Config

	if err := stepconf.Parse(&cfg); err != nil {
		fail("Issue with input: %s", err)
	}

	stepconf.Print(cfg)
	log.SetEnableDebugLog(cfg.VerboseLog)

	if err := cfg.validate(); err != nil {
		fail("Issue with input: %s", err)
	}

	content, err := cfg.downloadXml()
	if err != nil {
		fail("Unable to download Add-on: %@", err)
	}

	var addOn Addon
	xml.Unmarshal(content, &addOn)

	root, err := cfg.prepareExtraFolder()
	if err != nil {
		fail("Unable to create the extra folder: %@", err)
	}

	sdk_folder := addOn.Extra.createFolderStructure(root)

	for _, archive := range addOn.Extra.Archives {
		path, err := archive.downloadFile(sdk_folder)
		if err == nil {
			log.Debugf("Archive downloaded to %s", path)
			if err := unzipContent(path, sdk_folder); err != nil {
				fail("Fail to uncompress archive: %s", err.Error())
			}
		} else {
			log.Debugf("Error download archive: %@", err)
		}

		if _, err := os.Stat(path); err == nil {
			log.Debugf("Removing temp file at %s", path)
			os.Remove(path)
		} else {
			log.Debugf("Unable to get stat about %s: %@", path, err)
		}
	}

	log.Infof("Archives downloaded into %s", sdk_folder)

	c := exec.Command("envman", "add", "--key", "ADD_ON_SDK_PATH", "--value", sdk_folder)
	if err := c.Run(); err != nil {
		log.Errorf("Unable to update output ADD_ON_SDK_PATH")
	} else {
		log.Debugf("Output ADD_ON_SDK_PATH set to %s", sdk_folder)
	}
}
