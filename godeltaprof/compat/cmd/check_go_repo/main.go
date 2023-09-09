package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
)

const repo = "git@github.com:golang/go.git"
const mprof = "src/runtime/mprof.go"
const pprof = "src/runtime/pprof"
const repoDir = "go_repo"
const latestCommitsFile = "last_known_go_commits.json"

type Commits struct {
	Mprof string `json:"mprof"`
	Pprof string `json:"pprof"`
}

var latestCommits Commits
var current Commits

func main() {
	getRepo()
	loadLastKnownCommits()
	loadCurrentCommits()
	if latestCommits == current {
		log.Println("no new commits")
		return
	}
	writeLastKnownCommits()
}

func writeLastKnownCommits() {
	bs, err := json.MarshalIndent(&current, "", "  ")
	requireNoError(err, "marshal current commits")
	err = os.WriteFile(latestCommitsFile, bs, 0666)
	requireNoError(err, "write current commits")
}

func loadCurrentCommits() {
	current.Mprof = checkLatestCommit(mprof)
	current.Pprof = checkLatestCommit(pprof)

	log.Printf("current commits: %+v\n", current)
}

func checkLatestCommit(repoPath string) string {
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("git log -- %s | head -n 1", repoPath))
	cwd, err := os.Getwd()
	requireNoError(err, "cwd")
	cmd.Dir = path.Join(cwd, repoDir)
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err = cmd.Run()
	requireNoError(err, "checkLatestCommit "+repoPath)
	s := stdout.String()
	re := regexp.MustCompile("commit ([a-f0-9]{40})")
	match := re.FindStringSubmatch(s)
	if match == nil {
		requireNoError(fmt.Errorf("no commit found for %s %s", repoPath, s), "commit regex")
	}
	commit := match[1]
	log.Println("latest commit ", repoPath, commit)
	return commit
}

func loadLastKnownCommits() {
	bs, err := os.ReadFile(latestCommitsFile)
	requireNoError(err, "read known_commits.json")
	err = json.Unmarshal(bs, &latestCommits)
	requireNoError(err, "unmarshal known_commits.json")
	log.Printf("known commits: %+v\n", latestCommits)
}

func getRepo() {
	_, err := os.Stat(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			cmd := exec.Command("git", "clone", repo, repoDir)
			requireNoError(cmd.Run(), "git clone repo")
			log.Println("git clone done", cmd.ProcessState.ExitCode())
		} else {
			log.Fatal(err)
		}
	} else {
		log.Println("repo exists")
	}
}

func requireNoError(err error, msg string) {
	if err != nil {
		log.Fatal(msg, err)
	}
}
