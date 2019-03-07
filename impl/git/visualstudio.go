package git

import (
	"errors"
	"fmt"
	"github.com/go-resty/resty"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/prompt"
	"strings"
)

type visualStudio struct {
	config config.Config
	labels Labels
}

type vsProject struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Url         string `json:"url"`
	State       string `json:"state"`
	Revision    int    `json:"revision"`
	Visibility  string `json:"visibility"`
}

type vsProjectResponse struct {
	Value []vsProject `json:"value"`
	Count int         `json:"count"`
}

type vsRepository struct {
	Id            string    `json:"id"`
	Name          string    `json:"name"`
	Url           string    `json:"url"`
	RemoteUrl     string    `json:"remoteUrl"`
	SshUrl        string    `json:"sshUrl"`
	DefaultBranch string    `json:"defaultBranch"`
	Project       vsProject `json:"project"`
}

type vsRepositoriesResponse struct {
	Value []vsRepository `json:"value"`
	Count int            `json:"count"`
}

type vsProcess struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"isDefault"`
}

type vsProcessResponse struct {
	Value []vsProcess `json:"value"`
	Count int         `json:"count"`
}

type vsCreateProject struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Capabilities vsCreateProjectCapabilities `json:"capabilities"`
	Visibility   string                      `json:"visibility"`
}

type vsCreateProjectCapabilities struct {
	VersionControl  vsCreateProjectVersionControl  `json:"versioncontrol"`
	ProcessTemplate vsCreateProjectProcessTemplate `json:"processTemplate"`
}

type vsCreateProjectVersionControl struct {
	Type string `json:"sourceControlType"`
}

type vsCreateProjectProcessTemplate struct {
	Id string `json:"templateTypeId"`
}

type vsCreateProjectResponse struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Url    string `json:"url"`
}

type vsCreateRepository struct {
	Name    string                    `json:"name"`
	Project vsCreateRepositoryProject `json:"project"`
}

type vsCreateRepositoryProject struct {
	Id string `json:"id"`
}

var vsProjectVisibilities = []prompt.Option{
	{Id: "private", Name: "Private"},
	{Id: "public", Name: "Public"},
}

func newVisualStudio(config config.Config) gitRemote {
	return &visualStudio{
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

func (vs *visualStudio) getLabels() Labels {
	return vs.labels
}

func (vs *visualStudio) createGroup(args cli.Args) (string, error) {

	processResp, processErr := vs.execGet("_apis/process/processes?api-version=1.0", &vsProcessResponse{})
	if processErr != nil {
		return "", processErr
	}

	var selectedProcessId = ""
	var defaultProcessId = ""
	processOptions := funk.Map(processResp.(*vsProcessResponse).Value, func(process vsProcess) prompt.Option {
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
	visibility := prompt.Choice("\nSelect your project visibility :", vsProjectVisibilities, args.Get(2))
	projectProcessTemplateId := prompt.Choice("\nSelect your project process template :", processOptions, selectedProcessId)
	if projectProcessTemplateId == "" {
		projectProcessTemplateId = defaultProcessId
	}

	prompt.PrintNewLine()

	data := vsCreateProject{
		Name:        projectName,
		Description: projectDescription,
		Visibility:  visibility,
		Capabilities: vsCreateProjectCapabilities{
			VersionControl:  vsCreateProjectVersionControl{Type: "GIT"},
			ProcessTemplate: vsCreateProjectProcessTemplate{Id: projectProcessTemplateId},
		},
	}

	createResp, err := vs.execPost("_apis/projects?api-version=2.0-preview", data, &vsCreateProjectResponse{})

	if err != nil {
		prompt.PrintError("Cannot create project. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Project created")

	return createResp.(*vsCreateProjectResponse).Id, nil
}

func (vs *visualStudio) createRepository(args cli.Args) (string, error) {

	groups, err := vs.getGroups()

	groups = funk.Filter(groups, func(g gitGroup) bool { return funk.ContainsString(vs.config.Git.GroupIds, g.Id) }).([]gitGroup)

	idx := 0
	groupId := ""
	if len(groups) == 0 {
		prompt.PrintError("No projects available")
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

	data := vsCreateRepository{
		Name: projectName,
		Project: vsCreateRepositoryProject{
			Id: groupId,
		},
	}

	createResp, err := vs.execPost("_apis/git/repositories/?api-version=1.0", data, &vsRepository{})

	if err != nil {
		prompt.PrintError("Cannot create repository. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Repository created")

	return createResp.(*vsRepository).Id, nil
}

func (vs *visualStudio) getGroups() ([]gitGroup, error) {

	resp, err := vs.execGet("_apis/projects/?api-version=1.0", &vsProjectResponse{})
	if err != nil {
		return nil, err
	}

	return funk.Map(resp.(*vsProjectResponse).Value, vs.toGitGroup).([]gitGroup), nil
}

func (vs *visualStudio) getRepositories() ([]gitRepository, error) {

	var repos []gitRepository

	funk.ForEach(vs.config.Git.GroupIds, func(groupId string) {

		resp, err := vs.execGet(groupId+"/_apis/git/repositories/?api-version=1.0", &vsRepositoriesResponse{})
		if err == nil {
			repos = append(repos, funk.Map(resp.(*vsRepositoriesResponse).Value, vs.toGitRepo(groupId)).([]gitRepository)...)
		}
	})

	return repos, nil
}

func (vs *visualStudio) createReviewRequest(group string, folder string, from string, into string, title string, message string) (reviewRequest, error) {

	// TODO ( Implement )
	return reviewRequest{}, nil
}

func (vs *visualStudio) execGet(url string, resultType interface{}) (interface{}, error) {

	resp, err := resty.R().
		SetBasicAuth("", vs.config.Git.PrivateToken).
		SetResult(resultType).
		Get(vs.config.Git.BaseUrl + "/" + url)

	if err != nil {
		prompt.PrintError(resp.String())
	}

	return resp.Result(), err
}

func (vs *visualStudio) execPost(url string, data interface{}, resultType interface{}) (interface{}, error) {

	resp, err := resty.R().
		SetBasicAuth("", vs.config.Git.PrivateToken).
		SetResult(resultType).
		SetBody(data).
		Post(vs.config.Git.BaseUrl + "/" + url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return nil, errors.New(fmt.Sprintf("Cannot execute post request. Status code : %d. Message : %s", resp.StatusCode(), resp.String()))
	}

	return resp.Result(), nil
}

func (vs *visualStudio) toGitRepo(groupId string) interface{} {

	return func(repository vsRepository) gitRepository {
		return gitRepository{
			Id:                repository.Id,
			Name:              repository.Name,
			NameWithNamespace: repository.Project.Name + " / " + repository.Name,
			Path:              vs.normalize(repository.Name),
			PathWithNamespace: vs.normalize(repository.Project.Name) + "/" + vs.normalize(repository.Name),
			SshUrl:            repository.SshUrl,
			HttpUrl:           repository.RemoteUrl,
			DefaultBranch:     repository.DefaultBranch,
			Archived:          false,
			GroupId:           groupId,
		}
	}
}

func (vs *visualStudio) toGitGroup(project vsProject) gitGroup {

	return gitGroup{
		Id:   project.Id,
		Name: project.Name,
	}
}

func (vs *visualStudio) normalize(name string) string {
	return strings.ToLower(strings.Replace(name, " ", "-", -1))
}
