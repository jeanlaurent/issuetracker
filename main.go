package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

func newClient(authenticationToken string) *github.Client {
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authenticationToken},
	)
	tokenContext := oauth2.NewClient(oauth2.NoContext, tokenSource)
	return github.NewClient(tokenContext)
}

func getAllIssuesFromGithub(client *github.Client) []github.Issue {
	option := &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	var allIssues []github.Issue
	for {
		issues, response, err := client.Issues.ListByRepo("docker", "machine", option)
		if err != nil {
			fmt.Println(err.Error())
		}
		allIssues = append(allIssues, issues...)
		fmt.Println("got", len(issues), "issues")
		if response.NextPage == 0 {
			break
		}
		option.ListOptions.Page = response.NextPage
	}
	fmt.Printf("got %d issues\n", len(allIssues))
	return allIssues
}

func writeIssueToDisk(allIssues []github.Issue) {
	payload, err := json.Marshal(allIssues)
	if err != nil {
		fmt.Println(err.Error())
	}
	err = ioutil.WriteFile("issues.json", payload, 0644)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("Wrote issues.json")
}

func readIssuesFromDisk() []github.Issue {
	issuesData, err := ioutil.ReadFile("issues.json")
	if err != nil {
		fmt.Println(err.Error())
	}
	var allIssues []github.Issue
	err = json.Unmarshal(issuesData, &allIssues)
	if err != nil {
		fmt.Println(err.Error())
	}
	return allIssues
}

type myIssue struct {
	open   int
	closed int
}

type stat struct {
	day    string
	issues []*github.Issue
	open   int
	closed int
}

func (s stat) net() int {
	return s.open - s.closed
}

func newStat(day string, issues []*github.Issue) stat {
	nStat := stat{day, issues, 0, 0}
	for _, issue := range issues {
		if *issue.State == "closed" {
			nStat.closed++
		} else {
			nStat.open++
		}
	}
	return nStat
}

type stats []stat

func (a stats) Len() int           { return len(a) }
func (a stats) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a stats) Less(i, j int) bool { return a[i].day < a[j].day }

func display(name string, itemPerDay map[string][]*github.Issue) {
	fmt.Printf("%ss per day\n", name)
	var itemStat stats
	for day, list := range itemPerDay {
		itemStat = append(itemStat, newStat(day, list))
	}
	sort.Sort(itemStat)
	for _, stat := range itemStat {
		fmt.Printf("%s -> %d %s ( %+d ) -> ", stat.day, len(stat.issues), name, stat.net())
		for _, issue := range stat.issues {
			var duration int
			if *issue.State == CLOSED {
				duration = int(issue.CreatedAt.Sub(*issue.ClosedAt).Hours() / 24)
			} else {
				duration = int(time.Now().Sub(*issue.CreatedAt).Hours() / 24)
			}
			fmt.Printf("%s (%d),", *issue.State, duration)
		}
		fmt.Println()
	}
}

//No comment
const (
	CLOSED     = "closed"
	DATEFORMAT = "2006-01-02"
)

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("Need the GITHUB_TOKEN env variable.")
		os.Exit(3)
	}
	var allIssues []github.Issue
	if _, err := os.Stat("issues.json"); err != nil {
		fmt.Println("Retrieving remote issues")
		client := newClient(token)
		allIssues = getAllIssuesFromGithub(client)
		writeIssueToDisk(allIssues)
	} else {
		fmt.Println("Reading local issues")
		allIssues = readIssuesFromDisk()
	}
	typePullRequest := myIssue{}
	typeIssue := myIssue{}

	issuePerDay := make(map[string][]*github.Issue)
	prPerDay := make(map[string][]*github.Issue)

	for index := range allIssues {
		issue := &allIssues[index]
		var day string
		if issue.PullRequestLinks != nil {
			if *issue.State == CLOSED {
				typePullRequest.closed++
				day = issue.ClosedAt.Format(DATEFORMAT)
			} else {
				typePullRequest.open++
				day = issue.CreatedAt.Format(DATEFORMAT)
			}
			prPerDay[day] = append(prPerDay[day], issue)
		} else {
			if *issue.State == CLOSED {
				day = issue.ClosedAt.Format(DATEFORMAT)
				typeIssue.closed++
			} else {
				typeIssue.open++
				day = issue.CreatedAt.Format(DATEFORMAT)
			}
			issuePerDay[day] = append(issuePerDay[day], issue)
		}

	}
	fmt.Printf("pull requests\n")
	fmt.Printf("\topen %d\n", typePullRequest.open)
	fmt.Printf("\tclosed %d\n", typePullRequest.closed)

	fmt.Printf("issues\n")
	fmt.Printf("\topen %d\n", typeIssue.open)
	fmt.Printf("\tclosed %d\n", typeIssue.closed)

	display("Pull Request", prPerDay)
	display("Issue", issuePerDay)
}
