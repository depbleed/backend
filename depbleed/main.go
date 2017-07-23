package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/depbleed/backend/git"
	"github.com/depbleed/backend/persistence"
	depbleed "github.com/depbleed/go/go-depbleed"

	goji "goji.io"

	"strings"

	"goji.io/pat"
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

type backend struct {
	persistence persistence.DAO
}

func main() {

	persistence, err := persistence.NewMongo()

	if err != nil {
		fmt.Println("Can't initialize the database")
		panic(err.Error())
	}

	backend := &backend{
		persistence: persistence,
	}

	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "80")
	}

	fmt.Println("Serving on localhost", os.Getenv("PORT"))

	mux := goji.NewMux()
	mux.HandleFunc(pat.Get("/leaks/go/:user/:repo"), analyse(backend))
	mux.HandleFunc(pat.Get("/leaks/go/all/:skip/:limit"), allRepositories(backend))
	http.ListenAndServe(":"+os.Getenv("PORT"), mux)
}

func log(ip string, time string, method string, path string, code string, elasped string) {

	fmt.Printf(
		"%s [%s] \"%s %s\" %s (%s)\n",
		ip, time, method, path, code, elasped)
}

func analyse(b *backend) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		user := pat.Param(r, "user")
		repo := pat.Param(r, "repo")

		var repository persistence.Repository
		lastCommit := git.FetchLastCommit(user + "/" + repo)
		repository, err := b.persistence.FindRepo("github.com/" + user + "/" + repo)

		if err != nil {
			//Didn't find this repo; insert it
			repository = persistence.Repository{
				URL:      "github.com/" + user + "/" + repo,
				Language: "GO",
			}
			b.persistence.InsertRepo(repository)

		} else if len(repository.Analysis) != 0 && repository.Analysis[0].Hash == lastCommit {
			//This repo is up to date; return the last analyse
			respBody, err := json.MarshalIndent(repository, "", "  ")
			if err != nil {
				handleErrorRepo("Can't marshall repository", err, start, user, repo, r, w)
				return
			}
			log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/go/"+user+"/"+repo, "200", time.Since(start).String())
			ResponseWithJSON(w, respBody, http.StatusOK)
			return
		}

		//TODO: Determine repo is currently being analyzed

		git.CloneRepo(user + "/" + repo)

		analysis := &persistence.Analysis{
			Hash:  lastCommit,
			Leaks: []*persistence.Leak{},
			Time:  time.Now().Unix(),
		}
		runAnalysis(analysis, user, repo)

		//Append the analysis
		repository.Analysis = append(repository.Analysis, analysis)

		err = b.persistence.UpdateRepo(repository)
		if err != nil {
			handleErrorRepo("Can't update repository", err, start, user, repo, r, w)
			return
		}

		respBody, err := json.MarshalIndent(repository, "", "  ")
		if err != nil {
			handleErrorRepo("Can't marshall repo", err, start, user, repo, r, w)
			return
		}

		git.DeleteRepo(user)

		log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/go/"+user+"/"+repo, "200", time.Since(start).String())
		ResponseWithJSON(w, respBody, http.StatusOK)
		return
	}
}

func handleErrorRepo(errString string, err error, start time.Time, user string, repo string, r *http.Request, w http.ResponseWriter) {
	fmt.Println(errString, err.Error())
	log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/go/"+user+"/"+repo, "500", time.Since(start).String())
	ErrorWithJSON(w, "Something went wrong", 500)
	git.DeleteRepo(user)
}

func runAnalysis(analysis *persistence.Analysis, user string, repo string) {
	wd, _ := os.Getwd()
	//Get paths
	absPath, _ := filepath.Abs(wd)
	gopath := os.Getenv("GOPATH") // build.Default.GOPATH
	packagePath, _ := depbleed.GetPackagePath(gopath, absPath+"/repositories/"+user+"/"+repo)
	packageInfo, _ := depbleed.GetPackageInfo(absPath + "/repositories/" + user + "/" + repo)

	fmt.Println(absPath)
	fmt.Println(gopath)
	fmt.Println(packagePath)
	fmt.Println(packageInfo)

	//Compute leaks
	leaks := packageInfo.Leaks()
	for _, leak := range leaks {

		relPath, _ := filepath.Rel(wd, leak.Position.Filename)
		relPath = strings.Replace(leak.Position.Filename, absPath+"/repositories/", "", 1)

		//Append all the leaks
		analysis.Leaks = append(analysis.Leaks, &persistence.Leak{
			Message: strings.Replace(leak.Error(), "github.com/depbleed/backend/repositories/", "", 1),
			Column:  leak.Position.Column,
			Line:    leak.Position.Line,
			File:    relPath,
		})

	}
}

func allRepositories(b *backend) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		skip := pat.Param(r, "skip")
		limit := pat.Param(r, "limit")

		skipInt, errSkip := strconv.Atoi(skip)
		limitInt, errlimit := strconv.Atoi(limit)

		if errSkip != nil || errlimit != nil {
			fmt.Println("Can't convert to ints", errSkip.Error(), errlimit.Error())
			log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/all/"+skip+"/"+limit, "500", time.Since(start).String())
			ErrorWithJSON(w, "Skip & Limit are expected to be ints", 500)
			return
		}

		repos, err := b.persistence.FindAll(skipInt, limitInt)

		if err != nil {
			fmt.Println("Can't fetch repos", err.Error())
			log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/all/"+skip+"/"+limit, "500", time.Since(start).String())
			ErrorWithJSON(w, "Something went wrong", 500)
		}

		respBody, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			fmt.Println("Can't marshall", err.Error())
			log(r.RemoteAddr, time.Now().Format(time.RFC1123), "GET", "/leaks/all/"+skip+"/"+limit, "500", time.Since(start).String())
			ErrorWithJSON(w, "Something went wrong", 500)
			return
		}

		ResponseWithJSON(w, respBody, http.StatusOK)
	}
}
