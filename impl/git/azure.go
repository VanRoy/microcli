package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v3"
	"github.com/vanroy/microcli/impl/cmd"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/prompt"
)

type azure struct {
	config config.Config
	labels Labels
}

type azProject struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
	State       string `json:"state"`
	Revision    int    `json:"revision"`
	Visibility  string `json:"visibility"`
}

type azProjectResponse struct {
	Value []azProject `json:"value"`
	Count int         `json:"count"`
}

type azRepository struct {
	Id            string    `json:"id"`
	Name          string    `json:"name"`
	Url           string    `json:"url"`
	RemoteUrl     string    `json:"remoteUrl"`
	SshUrl        string    `json:"sshUrl"`
	WebUrl        string    `json:"webUrl"`
	DefaultBranch string    `json:"defaultBranch"`
	Project       azProject `json:"project"`
}

type azRepositoriesResponse struct {
	Value []azRepository `json:"value"`
	Count int            `json:"count"`
}

type azProcess struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"isDefault"`
}

type azProcessResponse struct {
	Value []azProcess `json:"value"`
	Count int         `json:"count"`
}

type azCreateProject struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Capabilities azCreateProjectCapabilities `json:"capabilities"`
	Visibility   string                      `json:"visibility"`
}

type azCreateProjectCapabilities struct {
	VersionControl  azCreateProjectVersionControl  `json:"versioncontrol"`
	ProcessTemplate azCreateProjectProcessTemplate `json:"processTemplate"`
}

type azCreateProjectVersionControl struct {
	Type string `json:"sourceControlType"`
}

type azCreateProjectProcessTemplate struct {
	Id string `json:"templateTypeId"`
}

type azCreateProjectResponse struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Url    string `json:"url"`
}

type azCreateRepository struct {
	Name    string                    `json:"name"`
	Project azCreateRepositoryProject `json:"project"`
}

type azCreateRepositoryProject struct {
	Id string `json:"id"`
}

type azCreatePullRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	SourceRefName string `json:"sourceRefName"`
	TargetRefName string `json:"targetRefName"`
	IsDraft       bool   `json:"isDraft"`
}

type azPullRequest struct {
	PullRequestId int          `json:"pullRequestId"`
	Repository    azRepository `json:"repository"`
	Url           string       `json:"url"`
	Status        string       `json:"status"`
}

var azProjectVisibilities = []prompt.Option{
	{Id: "private", Name: "Private"},
	{Id: "public", Name: "Public"},
}

func newAzure(config config.Config) gitRemote {
	return &azure{
		config: config,
		labels: Labels{
			GroupLabel:            "projet",
			GroupsLabel:           "projets",
			RepositoryLabel:       "repository",
			RepositoriesLabel:     "repositories",
			CreateGroupUsage:      "name [description] [visibility] [process]",
			CreateRepositoryUsage: "[project] name",
			CodeReviewRequest:     "pull requests",
		},
	}
}

func (az *azure) getLabels() Labels {
	return az.labels
}

func (az *azure) createGroup(args cli.Args) (string, error) {

	processResp, processErr := az.execGet("_apis/process/processes?api-version=7.1", &azProcessResponse{})
	if processErr != nil {
		return "", processErr
	}

	var selectedProcessId = ""
	var defaultProcessId = ""
	processOptions := funk.Map(processResp.(*azProcessResponse).Value, func(process azProcess) prompt.Option {
		var def = ""

		if args.Get(3) == process.Name {
			selectedProcessId = process.Id
		}

		if process.IsDefault {
			def = " (default)"
			defaultProcessId = process.Id
		}
		return prompt.Option{Id: process.Id, Name: fmt.Sprintf("%s : %s%s", process.Name, process.Description, def)}
	}).([]prompt.Option)

	projectName := prompt.Input("Enter your project name :", args.Get(0))
	projectDescription := prompt.Input("\nEnter your project description :", args.Get(1))
	visibility := prompt.Choice("\nSelect your project visibility :", azProjectVisibilities, args.Get(2))
	projectProcessTemplateId := prompt.Choice("\nSelect your project process template :", processOptions, selectedProcessId)
	if projectProcessTemplateId == "" {
		projectProcessTemplateId = defaultProcessId
	}

	prompt.PrintNewLine()

	data := azCreateProject{
		Name:        projectName,
		Description: projectDescription,
		Visibility:  visibility,
		Capabilities: azCreateProjectCapabilities{
			VersionControl:  azCreateProjectVersionControl{Type: "GIT"},
			ProcessTemplate: azCreateProjectProcessTemplate{Id: projectProcessTemplateId},
		},
	}

	createResp, err := az.execPost("_apis/projects?api-version=2.0-preview", data, &azCreateProjectResponse{})

	if err != nil {
		prompt.PrintErrorf("Cannot create project. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Project created")

	return createResp.(*azCreateProjectResponse).Id, nil
}

func (az *azure) createRepository(args cli.Args) (string, error) {

	groups, _ := az.getGroups()

	groups = funk.Filter(groups, func(g gitGroup) bool { return funk.ContainsString(az.config.Git.GroupIds, g.Id) }).([]gitGroup)

	idx := 0
	groupId := ""
	if len(groups) == 0 {
		prompt.PrintErrorf("No projects available")
		return "", errors.New("no projects available")
	} else if len(groups) == 1 {
		groupId = groups[0].Id
	} else {
		defaultProjectId := ""
		groupOptions := funk.Map(groups, func(group gitGroup) prompt.Option {
			if args.Get(0) == group.Name {
				defaultProjectId = group.Id
			}
			return prompt.Option{Id: group.Id, Name: group.Name}
		}).([]prompt.Option)

		groupId = prompt.Choice("Select your project :", groupOptions, defaultProjectId)
		idx = idx + 1
	}

	projectName := prompt.Input("\nEnter your repository name :", args.Get(idx))
	prompt.PrintNewLine()

	data := azCreateRepository{
		Name: projectName,
		Project: azCreateRepositoryProject{
			Id: groupId,
		},
	}

	createResp, err := az.execPost("_apis/git/repositories/?api-version=7.1", data, &azRepository{})

	if err != nil {
		prompt.PrintErrorf("Cannot create repository. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Repository created")

	return createResp.(*azRepository).Id, nil
}

func (az *azure) getGroups() ([]gitGroup, error) {

	resp, err := az.execGet("_apis/projects/?api-version=7.1", &azProjectResponse{})
	if err != nil {
		return nil, err
	}

	return funk.Map(resp.(*azProjectResponse).Value, az.toGitGroup).([]gitGroup), nil
}

func (az *azure) getRepositories() ([]gitRepository, error) {

	var repos []gitRepository

	funk.ForEach(az.config.Git.GroupIds, func(groupId string) {

		resp, err := az.execGet(groupId+"/_apis/git/repositories/?api-version=7.1", &azRepositoriesResponse{})
		if err == nil {
			repos = append(repos, funk.Map(resp.(*azRepositoriesResponse).Value, az.toGitRepo(groupId)).([]gitRepository)...)
		}
	})

	return repos, nil
}

func (az *azure) createReviewRequest(groupId string, repoId string, from string, into string, title string, message string, draft bool) (reviewRequest, error) {
	data := azCreatePullRequest{
		Title:         title,
		SourceRefName: "refs/heads/" + strings.TrimPrefix(from, "refs/heads/"),
		TargetRefName: "refs/heads/" + strings.TrimPrefix(into, "refs/heads/"),
		Description:   message,
		IsDraft:       draft,
	}

	azReview, err := az.execPost(groupId+"/_apis/git/repositories/"+repoId+"/pullrequests?api-version=7.1", data, &azPullRequest{})
	if err != nil {
		return reviewRequest{}, err
	}

	return az.toReviewRequest(azReview.(*azPullRequest)), err
}

func (az *azure) execGet(url string, resultType interface{}) (interface{}, error) {

	resp, err := az.authenticate(resty.New().R()).
		SetResult(resultType).
		Get(az.config.Git.BaseUrl + "/" + url)

	if err != nil {
		prompt.PrintError(resp.String())
	}

	return resp.Result(), err
}

func (az *azure) execPost(url string, data interface{}, resultType interface{}) (interface{}, error) {

	resp, err := az.authenticate(resty.New().R()).
		SetResult(resultType).
		SetBody(data).
		Post(az.config.Git.BaseUrl + "/" + url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return nil, fmt.Errorf("cannot execute post request. Status code : %d. Message : %s", resp.StatusCode(), resp.String())
	}

	return resp.Result(), nil
}

var azToken string

func (az *azure) authenticate(request *resty.Request) *resty.Request {

	if az.config.Git.AuthMode == "az-cli" {
		if azToken == "" {
			token, err := cmd.ExecCmd("az", []string{"account", "get-access-token", "--resource", "499b84ac-1321-427f-aa17-267ca6975798", "--query", "\"accessToken\"", "-o", "tsv"})
			if err != nil {
				prompt.PrintErrorf("Cannot retrieve authentication token via Azure CLI (%s)", err.Error())
				return request
			}
			azToken = token
		}

		request.SetHeader("Authorization", "Bearer "+azToken)
		return request
	}

	request.SetBasicAuth("", az.config.Git.PrivateToken)
	return request
}

func (az *azure) toGitRepo(groupId string) interface{} {

	return func(repository azRepository) gitRepository {
		return gitRepository{
			Id:                repository.Id,
			Name:              repository.Name,
			NameWithNamespace: repository.Project.Name + " / " + repository.Name,
			Path:              az.normalize(repository.Name),
			PathWithNamespace: az.normalize(repository.Project.Name) + "/" + az.normalize(repository.Name),
			SshUrl:            repository.SshUrl,
			HttpUrl:           repository.RemoteUrl,
			DefaultBranch:     strings.TrimPrefix(repository.DefaultBranch, "refs/heads/"),
			Archived:          false,
			GroupId:           groupId,
		}
	}
}

func (az *azure) toGitGroup(project azProject) gitGroup {

	return gitGroup{
		Id:   project.Id,
		Name: project.Name,
	}
}
func (az *azure) toReviewRequest(request *azPullRequest) reviewRequest {

	return reviewRequest{
		Id:        strconv.Itoa(request.PullRequestId),
		Url:       fmt.Sprintf("%s/pullrequest/%d", request.Repository.WebUrl, request.PullRequestId),
		State:     request.Status,
		Mergeable: "",
	}
}

func (az *azure) normalize(name string) string {
	if !az.config.Git.NormalizeName {
		return name
	}
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
