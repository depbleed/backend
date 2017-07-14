package backend

import (
	"encoding/json"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	depbleed "github.com/depbleed/go/go-depbleed"

	goji "goji.io"

	"goji.io/pat"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func ErrorWithJSON(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, "{message: %q}", message)
}

func ResponseWithJSON(w http.ResponseWriter, json []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(json)
}

type Repository struct {
	URL      string     `json:"url"`
	Analysis []Analysis `json:"analysis"`
}

type Analysis struct {
	Leaks []Leak `json:"leaks"`
	Hash  string `json:"hash"`
}

type Leak struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Path    string `json:"path"`
}

func main() {

	port := os.Getenv("PORT")
	mongodCred := &mgo.Credential{
		Username: os.Getenv("MONGOD_USER"),
		Password: os.Getenv("MONGOD_PASSWORD"),
	}
	session, err := mgo.Dial(os.Getenv("MONGOD_ADDRESS"))
	if err != nil {
		panic(err)
	}
	defer session.Close()
	session.Login(mongodCred)

	session.SetMode(mgo.Monotonic, true)
	ensureIndex(session)

	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/leaks/all"), allRepositories(session))
	mux.HandleFunc(pat.Get("/leaks/:repo"), analyse(session))
	http.ListenAndServe("localhost:"+port, mux)
}

func ensureIndex(s *mgo.Session) {
	session := s.Copy()
	defer session.Close()

	c := session.DB("store").C("repository")

	index := mgo.Index{
		Key:        []string{"url"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	err := c.EnsureIndex(index)
	if err != nil {
		panic(err)
	}
}

func fetchLastCommit(repo string) string {
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

	}
	type githubAPIResponse struct {
		ref    string
		url    string
		object struct {
			sha string
			url string
		}
	}

	var gAPIResponse githubAPIResponse

	err = json.Unmarshal(body, &gAPIResponse)
	return gAPIResponse.object.sha
}

func cloneRepo(repo string) {
	wd, _ := os.Getwd()
	//Create the directories for the repo
	cmdArgs := []string{wd + "repositories/" + repo}
	cmd := exec.Command("mkdir -p", cmdArgs...)
	if _, err := cmd.Output(); err != nil {
		log.Panic("There was an error running git log command: ", err)
	}

	//Clone the repo
	cmdArgs = []string{"clone", "--depth=1", "https://github.com/" + repo, wd + "repositories/" + repo}
	cmd = exec.Command("git", cmdArgs...)
	if _, err := cmd.Output(); err != nil {
		log.Panic("There was an error running git log command: ", err)
	}
}

func analyse(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		repo := pat.Param(r, "repo")

		var repository Repository
		lastCommit := fetchLastCommit(repo)
		c := session.DB(os.Getenv("DB_NAME")).C("repository")
		err := c.Find(bson.M{"repo": repo}).One(&repository)

		//Didn't find this repo; insert it
		if repository.URL == "" {
			repository.URL = "github.com" + repo
			c.Insert(repository)
		}

		//This repo is up to date; return the last analyse
		if len(repository.Analysis) != 0 && repository.Analysis[0].Hash == lastCommit {
			respBody, err := json.MarshalIndent(repository, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			ResponseWithJSON(w, respBody, http.StatusOK)
			return
		}

		wd, err := os.Getwd()

		analysis := Analysis{
			Hash: lastCommit,
		}

		cloneRepo(repo)

		//Get paths
		absPath, err := filepath.Abs(wd)
		gopath := build.Default.GOPATH
		packagePath, err := depbleed.GetPackagePath(gopath, absPath+"repositories/"+repo)
		packageInfo, err := depbleed.GetPackageInfo(packagePath)

		//Compute leaks
		leaks := packageInfo.Leaks()
		for _, leak := range leaks {
			relPath, err := filepath.Rel(wd, leak.Position.Filename)

			if err != nil {
				relPath = leak.Position.Filename
			}

			//Append all the leaks
			analysis.Leaks = append(analysis.Leaks, Leak{
				Message: fmt.Sprintf("%s:%d:%d: %s\n", relPath, leak.Position.Line, leak.Position.Column, leak),
				Column:  leak.Position.Column,
				Line:    leak.Position.Line,
				Path:    relPath,
			})
		}

		//Append the leak
		repository.Analysis = append(repository.Analysis, analysis)

		//Update the repo
		c.Update(bson.M{"repo": repo}, &repository)
		respBody, err := json.MarshalIndent(repository, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		ResponseWithJSON(w, respBody, http.StatusOK)
		return
	}
}

func allRepositories(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("store").C("repository")

		var repositories []Repository
		//TODO: Can we fetch only the last Analysis or a leak count ?
		err := c.Find(bson.M{}).All(&repositories)
		if err != nil {
			ErrorWithJSON(w, "Database error", http.StatusInternalServerError)
			log.Println("Failed get all repositories: ", err)
			return
		}

		respBody, err := json.MarshalIndent(repositories, "", "  ")
		if err != nil {
			log.Fatal(err)
		}

		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
