package main

import (
	"errors"
	"testing"

	"github.com/depbleed/backend/persistence"
)

func TestLog(t *testing.T) {

	log("a", "b", "c", "d", "e", "f")
}

type mockDAO struct{}

func (mg *mockDAO) UpdateRepo(repository persistence.Repository) error {

	if repository.URL == "" {
		return errors.New("bla")
	}
	return nil
}
func (mg *mockDAO) InsertRepo(repository persistence.Repository) error {
	if repository.URL == "" {
		return errors.New("bla")
	}
	return nil
}
func (mg *mockDAO) FindRepo(url string) (persistence.Repository, error) {
	if url == "" {
		return persistence.Repository{}, errors.New("bla")
	}
	return persistence.Repository{}, nil
}

func (mg *mockDAO) FindAll(skip int, limit int) ([]persistence.Repository, error) {

	if skip == -1 {
		return nil, errors.New("bla")
	}

	return []persistence.Repository{}, nil
}
