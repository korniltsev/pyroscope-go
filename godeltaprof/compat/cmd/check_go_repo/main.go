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
const myRemote = "origin"

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

var shMy = sh{}
var shGo = sh{wd: getRepoDir()}

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
		commits, _ := shGo.sh(fmt.Sprintf("git log  %s..%s -- %s", known.Mprof, current.Mprof, mprof))
		msg += "```\n"
		msg += commits
		msg += "\n"
		msg += "```\n"
	}
	//git log  1c0035401358c8bfc2ff646b1d950da5fcd6b355..a7c3de705287d56e3bea8a84ed9a56e4102d3f39 -- src/runtime/mprof.go

	if current.Pprof != known.Pprof {
		msg += pprof
		msg += "\n"
		msg += "last known [" + known.Pprof + "](" + commitUrl + known.Pprof + ")\n"
		msg += "current    [" + current.Pprof + "](" + commitUrl + current.Pprof + ")\n"

		commits, _ := shGo.sh(fmt.Sprintf("git log  %s..%s -- %s", known.Pprof, current.Pprof, pprof))
		msg += "```\n"
		msg += commits
		msg += "\n"
		msg += "```\n"
	}
	log.Println(msg)

	prs := getPullRequests()
	found := -1
out:
	for i, pr := range prs {
		for j := range pr.Labels {
			if pr.Labels[j].Name == label {
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
		updatePR(msg, prs[found])
	}

}

func updatePR(msg string, request PullRequest) {

}

func createPR(msg string) {
	branchName := fmt.Sprintf("check_go_repo_%d", time.Now().Unix())
	commitMessage := fmt.Sprintf("chore(check_go_repo): update %s", latestCommitsFile)

	shMy.sh(fmt.Sprintf("git checkout -b %s", branchName))
	shMy.sh(fmt.Sprintf("git ci -am '%s'", commitMessage))
	shMy.sh(fmt.Sprintf("git push %s %s", myRemote, branchName))

	shMy.sh(fmt.Sprintf("gh pr create --title '%s' --body '%s' --label '%s' ", commitMessage, msg, label))

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
	s, _ := shGo.sh(fmt.Sprintf("git log -- %s | head -n 1", repoPath))
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
			shMy.sh(fmt.Sprintf("git clone %s %s", goRepoURL, repoDir))
			log.Println("git clone done")
		} else {
			log.Fatal(err)
		}
	} else {
		log.Println("repo exists")
	}

	shGo.sh("git checkout master && git pull")
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

type PullRequest struct {
	BaseRefName string       `json:"baseRefName"`
	HeadRefName string       `json:"headRefName"`
	Id          string       `json:"id"`
	Labels      []IssueLabel `json:"labels"`
	Number      int          `json:"number"`
}

type IssueLabel struct {
	Name string `json:"name"`
}

func getPullRequests() []PullRequest {
	s := sh{}
	stdout, _ := s.sh("gh pr list --json 'id,number,labels,baseRefName,headRefName' -R " + myRepoURL)
	var prs []PullRequest
	err := json.Unmarshal([]byte(stdout), &prs)
	requireNoError(err, "unmarshal prs")
	log.Println("prs", prs)
	return prs
}

type sh struct {
	wd string
}

func (s *sh) sh(sh string) (string, string) {
	return s.cmd("/bin/sh", "-c", sh)
}

func (s *sh) cmd(cmdArgs ...string) (string, string) {
	log.Printf("cmd %s\n", strings.Join(cmdArgs, " "))
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = s.wd
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	fmt.Println(stdout.String())
	fmt.Println(stderr.String())
	requireNoError(err, strings.Join(cmdArgs, " "))
	return stdout.String(), stderr.String()
}
