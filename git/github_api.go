package git

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

//FetchLastCommit fetches the last commit hash of repo
func FetchLastCommit(repo string) string {
	//Query github to get the last commit
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	var netClient = &http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}

	response, _ := netClient.Get("https://api.github.com/repos/" + repo + "/git/refs/heads/master")
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)

	if err != nil {
		panic(err.Error())
	}
	type githubAPIResponse struct {
		Ref    string `json:"ref"`
		URL    string `json:"url"`
		Object struct {
			Sha  string `json:"sha"`
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"object"`
	}

	var gAPIResponse githubAPIResponse

	err = json.Unmarshal(body, &gAPIResponse)
	if err != nil {
		panic(err.Error())
	}
	return gAPIResponse.Object.Sha
}

//CloneRepo clones the repo locally
func CloneRepo(repo string) {
	wd, _ := os.Getwd()
	//Create the directories for the repo
	cmdArgs := []string{"-p", wd + "/repositories/" + repo}
	cmd := exec.Command("mkdir", cmdArgs...)
	if _, err := cmd.Output(); err != nil {
		log.Panic("There was an error creating the folders", err)
	}

	//Clone the repo
	cmdArgs = []string{"clone", "--depth=1", "https://github.com/" + repo, wd + "/repositories/" + repo}
	cmd = exec.Command("git", cmdArgs...)
	if _, err := cmd.Output(); err != nil {
		log.Panic("There was an error running git clone command: ", err)
	}
}

//DeleteRepo deletes a local repo
func DeleteRepo(repo string) {
	wd, _ := os.Getwd()
	//Delete the directories for the repo
	cmdArgs := []string{"-rf", wd + "/repositories/" + repo}
	cmd := exec.Command("rm", cmdArgs...)
	if _, err := cmd.Output(); err != nil {
		log.Panic("There was an error running deleting folders: ", err)
	}
}
