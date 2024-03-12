package utils

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime"

	"github.com/sirupsen/logrus"
)

/*
	const data = {
	      current_version: "0.4.0",
	      current_version_published: "11 Feb 2024",
	      download_link: "https://github.com/logdyhq/logdy-core/releases/tag/v0.3.0",
	      excerpt: "What's new in version 0.4.0"
	    };
*/

type UpdateResponse struct {
	CurrentVersion          string `json:"current_version"`
	CurrentVersionPublished string `json:"current_version_published"`
	DownloadLink            string `json:"download_link"`
	BlogLink                string `json:"blog_link"`
	Excerpt                 string `json:"excerpt"`
}

func checkUpdates(version string) (UpdateResponse, error) {
	update := UpdateResponse{}
	resp, err := http.Get("https://update.logdy.dev?version=" + version)
	if err != nil {
		return update, err
	}

	defer resp.Body.Close()
	//Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return update, err
	}

	err = json.Unmarshal(body, &update)

	return update, err
}

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"

func init() {
	if runtime.GOOS == "windows" {
		Reset = ""
		Red = ""
		Green = ""
		Yellow = ""
	}
}

func CheckUpdatesAndPrintInfo(version string) {
	update, err := checkUpdates(version)

	if err != nil {
		Logger.WithField("error", err).Error("Error while checking for Logdy updates")
		return
	}

	if update.CurrentVersion == version {
		Logger.WithFields(logrus.Fields{
			"current_version": version,
			"latest_version":  update.CurrentVersion,
		}).Debug("No updates detected")
		return
	}

	Logger.WithFields(logrus.Fields{
		"response":        update,
		"current_version": version,
		"latest_version":  update.CurrentVersion,
	}).Debug("New version available")

	Logger.Info(Yellow + "----------------------------------------------------------")
	Logger.Info(Yellow + ">                NEW LOGDY VERSION AVAILABLE              ")
	Logger.Info(Yellow + "> Version: " + update.CurrentVersion)
	Logger.Info(Yellow + "> Date published: " + update.CurrentVersionPublished)
	Logger.Info(Yellow + "> Download: " + update.DownloadLink)
	Logger.Info(Yellow + "> Blog: " + update.BlogLink)

	if update.Excerpt != "" {
		Logger.Info(Yellow + "> " + update.Excerpt)
	}

	Logger.Info(Yellow + "----------------------------------------------------------" + Reset)
}
