package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/badkaktus/gorocket"
)

type SingleMilestone struct {
	ID          int         `json:"id"`
	Iid         int         `json:"iid"`
	GroupID     int         `json:"group_id"`
	Title       string      `json:"title"`
	Description interface{} `json:"description"`
	State       string      `json:"state"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	DueDate     string      `json:"due_date"`
	StartDate   string      `json:"start_date"`
	WebURL      string      `json:"web_url"`
}

type SingleIssue struct {
	ID        int    `json:"id"`
	Iid       int    `json:"iid"`
	ProjectID int    `json:"project_id"`
	Title     string `json:"title"`
	// Description        string          `json:"description"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	ClosedAt  time.Time `json:"closed_at"`
	ClosedBy  ClosedBy  `json:"closed_by"`
	Labels    []string  `json:"labels"`
	// Milestone          SingleMilestone `json:"milestone"`
	// Assignees          []Assignees     `json:"assignees"`
	// Author             Author          `json:"author"`
	// Assignee           Assignee        `json:"assignee"`
	UserNotesCount     int `json:"user_notes_count"`
	MergeRequestsCount int `json:"merge_requests_count"`
	Upvotes            int `json:"upvotes"`
	Downvotes          int `json:"downvotes"`
	// DueDate            string          `json:"due_date"`
	// Confidential       bool            `json:"confidential"`
	// DiscussionLocked   interface{}     `json:"discussion_locked"`
	WebURL    string    `json:"web_url"`
	TimeStats TimeStats `json:"time_stats"`
}

type TimeStats struct {
	TimeEstimate        int         `json:"time_estimate"`
	TotalTimeSpent      int         `json:"total_time_spent"`
	HumanTimeEstimate   interface{} `json:"human_time_estimate"`
	HumanTotalTimeSpent interface{} `json:"human_total_time_spent"`
}

type ClosedBy struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	State     string `json:"state"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

var glURL, glToken, rocketURL, rocketUser, rocketPass, rocketMsg, rocketChannel *string
var glGroupId *int

var client *http.Client
var wg sync.WaitGroup
var rocketClient *gorocket.Client

func main() {
	glURL = flag.String("gitlaburl", "", "GitLab URL")
	glToken = flag.String("token", "", "GitLab Private Token")
	glGroupId = flag.Int("group", 0, "GitLab Group ID")
	rocketURL = flag.String("rocketurl", "", "RocketChat URL")
	rocketUser = flag.String("user", "", "RocketChat User")
	rocketPass = flag.String("pass", "", "RocketChat Password")
	rocketChannel = flag.String("channel", "", "RocketChat channel to post")
	rocketMsg = flag.String("msg", "", "RocketChat message that will be sent to the channel")
	flag.Parse()

	client = &http.Client{}

	rocketClient = gorocket.NewClient(*rocketURL)

	payload := gorocket.LoginPayload{
		User:     *rocketUser,
		Password: *rocketPass,
	}

	loginResp, _ := rocketClient.Login(&payload)
	log.Printf("Rocket login status: %s", loginResp.Status)
	if loginResp.Message != "" {
		log.Printf("Rocket login response message: %s", loginResp.Message)
	}

	url := fmt.Sprintf("%s/api/v4/groups/%d/milestones?state=active", *glURL, *glGroupId)

	body, _ := sendReq(url, http.MethodGet, "")

	allML := []SingleMilestone{}

	_ = json.Unmarshal(body, &allML)

	for _, v := range allML {
		dueDate, _ := time.Parse("2006-01-02", v.DueDate)

		if time.Now().Before(dueDate) {
			continue
		}
		log.Printf("Work with milestone from %s to %s", v.StartDate, v.DueDate)

		wg.Add(1)

		go func() {
			issues, err := getIssuesInMilestone(v.ID)

			if err != nil {
				log.Fatal("something wrong")
			}

			if len(issues) == 0 {
				wg.Done()
				return
			}

			percent, err := percentIssuesOfClosed(issues)

			if err != nil {
				log.Fatal("something wrong one more time")
			}

			if percent == 100 {
				closeMilestone(v.ID)
			}

			wg.Done()
		}()

		wg.Wait()
	}
}

func getIssuesInMilestone(mlID int) ([]SingleIssue, error) {
	url := fmt.Sprintf("%s/api/v4/groups/%d/milestones/%d/issues", *glURL, *glGroupId, mlID)

	body, _ := sendReq(url, http.MethodGet, "")

	allIssuesInML := []SingleIssue{}

	json.Unmarshal(body, &allIssuesInML)

	log.Printf("issues in milestone %d - %d items", mlID, len(allIssuesInML))

	return allIssuesInML, nil
}

func percentIssuesOfClosed(issues []SingleIssue) (int, error) {

	closedIssues := 0

	var percent float32

	for _, v := range issues {
		if v.State == "closed" {
			closedIssues++
		}
	}

	percent = (float32(closedIssues) / float32(len(issues))) * 100

	intPercent := int(percent)

	log.Printf("%d%% closed", intPercent)

	return intPercent, nil
}

func closeMilestone(mlID int) {
	url := fmt.Sprintf("%s/api/v4/groups/%d/milestones/%d", *glURL, *glGroupId, mlID)

	body, _ := sendReq(url, http.MethodPut, "{\"state_event\": \"close\"}")

	closeMLResponse := SingleMilestone{}

	json.Unmarshal(body, &closeMLResponse)

	if closeMLResponse.State == "closed" {
		log.Printf("Close milestone: %v", closeMLResponse.Title)
		opt := gorocket.Message{
			Text:    fmt.Sprintf(*rocketMsg, closeMLResponse.Title),
			Channel: *rocketChannel,
		}
		hresp, err := rocketClient.PostMessage(&opt)

		log.Printf("PostMessage response status: %v", hresp.Success)

		if err != nil || hresp.Success == false {
			log.Printf("Sending message to Rocket.Chat error")
		}
	}
}

func sendReq(url, method, param string) ([]byte, error) {
	req, err := http.NewRequest(method, url, strings.NewReader(param))
	req.Header.Add("Private-Token", "aHCvBPtN9VYystdpHznV")
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Printf("error in url: %s [%s]", url, method)
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)

	return body, err
}
