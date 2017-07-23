package persistence

import (
	"os"
	"time"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Repository represents an analyzed repository
type Repository struct {
	URL      string      `json:"url"`
	Analysis []*Analysis `json:"analysis"`
	Language string
}

//Analysis represents a leak analysis
type Analysis struct {
	Leaks []*Leak `json:"leaks"`
	Hash  string  `json:"hash"`
	Time  int64   `json:"timestamp"`
}

//Leak represents one dependency leak
type Leak struct {
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Path    string `json:"path"`
}

//mongo is a DAO implementation for MongoDB
type mongo struct {
	dbName      string
	dbUser      string
	dbPassword  string
	dbAddress   string
	session     *mgo.Session
	credentials *mgo.Credential
}

//DAO defines the interface for a repository DAO
type DAO interface {
	UpdateRepo(repository Repository) error
	InsertRepo(repository Repository) error
	FindRepo(url string) (Repository, error)
	FindAll(skip int, limit int) ([]Repository, error)
}

//NewMongo returns a DAO implementation for mongo
func NewMongo() (*mongo, error) {

	mg := &mongo{
		dbName:     os.Getenv("MONGOD_DB"),
		dbUser:     os.Getenv("MONGOD_USER"),
		dbPassword: os.Getenv("MONGOD_PW"),
		dbAddress:  os.Getenv("MONGOD_URL"),
		credentials: &mgo.Credential{
			Username: os.Getenv("MONGOD_USER"),
			Password: os.Getenv("MONGOD_PW"),
		},
	}

	mgDBDialInfo := &mgo.DialInfo{
		Addrs:    []string{mg.dbAddress},
		Timeout:  60 * time.Second,
		Database: mg.dbName,
		Username: mg.dbUser,
		Password: mg.dbPassword,
	}

	session, err := mgo.DialWithInfo(mgDBDialInfo)

	if err != nil {
		return nil, err
	}

	mg.session = session
	err = mg.session.Login(mg.credentials)

	if err != nil {
		return nil, err
	}

	mg.session.SetMode(mgo.Monotonic, true)
	mg.ensureIndex()
	return mg, nil
}

func (mg *mongo) ensureIndex() {
	session := mg.session.Copy()
	defer session.Close()

	c := session.DB(mg.dbName).C("repository")

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

//UpdateRepo update a repo
func (mg *mongo) UpdateRepo(repository Repository) error {
	session := mg.session.Copy()
	defer session.Close()
	c := session.DB(mg.dbName).C("repository")
	return c.Update(bson.M{"url": repository.URL}, &repository)
}

//InsertRepo inserts a repo
func (mg *mongo) InsertRepo(repository Repository) error {
	session := mg.session.Copy()
	defer session.Close()
	c := session.DB(mg.dbName).C("repository")
	return c.Insert(repository)
}

//FindRepo finds a repo
func (mg *mongo) FindRepo(url string) (Repository, error) {
	session := mg.session.Copy()
	defer session.Close()

	var repository Repository

	c := session.DB(mg.dbName).C("repository")
	err := c.Find(bson.M{"url": url}).One(&repository)

	return repository, err
}

//FindAll retuns all the repo
func (mg *mongo) FindAll(skip int, limit int) ([]Repository, error) {
	session := mg.session.Copy()
	defer session.Close()

	repositories := []Repository{}

	c := session.DB(mg.dbName).C("repository")

	err := c.Find(bson.M{}).Skip(skip).Limit(limit).All(&repositories)
	return repositories, err
}
