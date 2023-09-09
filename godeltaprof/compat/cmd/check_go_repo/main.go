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
	"strings"
	"time"
)

const goRepoURL = "git@github.com:golang/go.git"

// todo change these to grafana
const myRepoURL = "https://github.com/korniltsev/pyroscope-go"
const myRemote = "korniltsev"

const mprof = "src/runtime/mprof.go"
const pprof = "src/runtime/pprof"
const repoDir = "go_repo"
const latestCommitsFile = "last_known_go_commits.json"
const label = "godeltaprof: check_go_repo"

type Commits struct {
	Mprof string `json:"mprof"`
	Pprof string `json:"pprof"`
}

var known Commits
var current Commits

func main() {
	getRepo()
	loadLastKnownCommits()
	loadCurrentCommits()
	if known == current {
		log.Println("no new commits")
		return
	}
	writeLastKnownCommits()
	createOrUpdatePR()
}

func createOrUpdatePR() {
	msg := ""
	const commitUrl = "https://github.com/golang/go/commit/"
	if current.Mprof != known.Mprof {
		msg += mprof
		msg += "\n"
		msg += "last known [" + known.Mprof + "](" + commitUrl + known.Mprof + ")\n"
		msg += "current    [" + current.Mprof + "](" + commitUrl + current.Mprof + ")\n"
	}

	if current.Pprof != known.Pprof {
		msg += pprof
		msg += "\n"
		msg += "last known [" + known.Pprof + "](" + commitUrl + known.Pprof + ")\n"
		msg += "current    [" + current.Pprof + "](" + commitUrl + current.Pprof + ")\n"
	}
	log.Println(msg)

	prs := getPRS()
	found := -1
out:
	for i, pr := range prs {
		for j := range pr.Labels {
			if pr.Labels[j] == label {
				found = i
				break out
			}
		}
	}
	if found == -1 {
		log.Println("existing PR not found, creating a new one")
		createPR(msg)
	} else {
		log.Printf("found existing PR %+v. updating.", prs[found])
	}

}

func createPR(msg string) {
	// create a branch
	branchName := fmt.Sprintf("check_go_repo_%d", time.Now().Unix())
	commitMessage := fmt.Sprintf("chore(check_go_repo): update %s", latestCommitsFile)
	sh := sh{}
	sh.sh(fmt.Sprintf("git checkout -b %s", branchName))
	sh.sh(fmt.Sprintf("git ci -am '%s'", commitMessage))
	//sh.sh(fmt.Sprintf("git push %s %s", myRemote, branchName))

	// create pR
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
	cmd.Dir = getRepoDir()
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err := cmd.Run()
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
	err = json.Unmarshal(bs, &known)
	requireNoError(err, "unmarshal known_commits.json")
	log.Printf("known commits: %+v\n", known)
}

func getRepo() {
	_, err := os.Stat(repoDir)
	if err != nil {
		if os.IsNotExist(err) {
			cmd := exec.Command("git", "clone", goRepoURL, repoDir)
			requireNoError(cmd.Run(), "git clone repo")
			log.Println("git clone done", cmd.ProcessState.ExitCode())
		} else {
			log.Fatal(err)
		}
	} else {
		log.Println("repo exists")
	}

	cmd := exec.Command("/bin/sh", "-c", "git checkout master && git pull")
	cmd.Dir = getRepoDir()
	requireNoError(cmd.Run(), "git pull")
	log.Println("git pull done")
}

func requireNoError(err error, msg string) {
	if err != nil {
		log.Fatalf("msg %s err %v", msg, err)
	}
}

func getRepoDir() string {
	cwd, err := os.Getwd()
	requireNoError(err, "cwd")
	return path.Join(cwd, repoDir)
}

type PR struct {
	BaseRefName string   `json:"baseRefName"`
	HeadRefName string   `json:"headRefName"`
	Id          string   `json:"id"`
	Labels      []string `json:"labels"`
	Number      int      `json:"number"`
}

func getPRS() []PR {
	cmd := exec.Command("gh", "pr", "list", "--json", "id,number,labels,baseRefName,headRefName",
		"-R", myRepoURL)
	cmd.Dir = getRepoDir()
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err := cmd.Run()
	requireNoError(err, "gh pr list")
	var prs []PR
	err = json.Unmarshal(stdout.Bytes(), &prs)
	requireNoError(err, "unmarshal prs")
	log.Println("prs", prs)
	return prs
}

type sh struct {
	wd string
}

func (s *sh) sh(sh string) string {
	return s.cmd("/bin/sh", "-c", sh)
}

func (s *sh) cmd(cmdArgs ...string) string {
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	stdout := bytes.NewBuffer(nil)
	//stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	//cmd.Stderr = stderr
	err := cmd.Run()
	requireNoError(err, strings.Join(cmdArgs, " "))
	//fmt.Println(stderr.String())
	return stdout.String()
}
