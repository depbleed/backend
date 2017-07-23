package git

import (
	"fmt"
	"os"
	"testing"
)

func TestFetchLastCommit(t *testing.T) {

	testCases := []struct {
		Repo string
	}{
		{
			Repo: "depbleed/go",
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%s", testCase.Repo), func(t *testing.T) {

			commit := FetchLastCommit("depbleed/go")

			if commit == "" {
				t.Errorf("expected commit to be not null; got %s", commit)
			}

		})
	}
}

func TestCloneDeleteRepo(t *testing.T) {

	testCases := []struct {
		Repo string
	}{
		{
			Repo: "depbleed/go",
		},
	}

	wd, _ := os.Getwd()

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%s", testCase.Repo), func(t *testing.T) {

			CloneRepo("depbleed/go")
			_, err := os.Stat(wd + "/repositories/" + testCase.Repo)

			if err != nil {
				t.Errorf("expected repo to be created %s", err.Error())
			}

			DeleteRepo("depbleed/go")

			_, err = os.Stat(wd + "/repositories/" + testCase.Repo)

			if err == nil {
				t.Errorf("expected repo to be deleted %s", err.Error())
			}
		})
	}

}
