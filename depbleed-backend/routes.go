package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

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
	URL string `json:"url"`
}

type Analysis struct {
	Leaks []Leak `json:"leaks"`
	Hash  string `json:"hash"`
}

type Leak struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Path    int    `json:"path"`
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

func analyse(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		repo := pat.Param(r, "repo")
		packageInfo, err := depbleed.GetPackageInfo(repo)
		if err != nil {
			log.Fatal(err)
		}

		packageInfo.Leaks()

		//TODO:
		//Check if current leaks are for the last github commit
		//If not; pull repo and check leaks
		//save leaks in mongod
		//return json
	}
}

func allRepositories(s *mgo.Session) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		session := s.Copy()
		defer session.Close()

		c := session.DB("store").C("repository")

		var repositories []Repository
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
