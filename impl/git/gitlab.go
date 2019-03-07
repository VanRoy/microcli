package git

import (
	"errors"
	"fmt"
	"github.com/go-resty/resty"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/prompt"
	"strconv"
)

type gitLab struct {
	config config.Config
	labels Labels
	apiUrl string
}

type glGroup struct {
	Id                   int    `json:"id"`
	Name                 string `json:"name"`
	Path                 string `json:"path"`
	Description          string `json:"description"`
	Visibility           string `json:"visibility"`
	LfsEnabled           bool   `json:"lfs_enabled"`
	AvatarUrl            string `json:"avatar_url"`
	WebUrl               string `json:"web_url"`
	RequestAccessEnabled bool   `json:"request_access_enabled"`
	FullName             string `json:"full_name"`
	FullPath             string `json:"full_path"`
	ParentId             int    `json:"parent_id"`
}

type glProject struct {
	Id                int    `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	Description       string `json:"description"`
	NameWithNamespace string `json:"name_with_namespace"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebUrl            string `json:"web_url"`
	HttpUrl           string `json:"http_url_to_repo"`
	SshUrl            string `json:"ssh_url_to_repo"`
	DefaultBranch     string `json:"default_branch"`
	Archived          bool   `json:"archived"`
}

var glVisibilityOptions = []prompt.Option{
	{Id: "public", Name: "Public"},
	{Id: "private", Name: "Private"},
	{Id: "internal", Name: "Internal"},
}

func newGitLab(config config.Config) gitRemote {
	return &gitLab{
		config: config,
		labels: Labels{
			GroupLabel:            "group",
			GroupsLabel:           "groups",
			RepositoryLabel:       "project",
			RepositoriesLabel:     "projects",
			CreateGroupUsage:      "name path [description] [visibility]",
			CreateRepositoryUsage: "[group] name [description] [visibility]",
			CodeReviewRequest:     "merge request",
		},
		apiUrl: config.Git.BaseUrl + "/api/v4",
	}
}

func (gl *gitLab) getLabels() Labels {
	return gl.labels
}

func (gl *gitLab) createGroup(args cli.Args) (string, error) {

	groupName := prompt.Input("Enter your group name :", args.Get(0))
	groupPath := prompt.Input("\nEnter your group path :", args.Get(1))
	groupDescription := prompt.Input("\nEnter your group description :", args.Get(2))
	groupVisibility := prompt.Choice("\nSelect your group visibility ( default: private ) :", glVisibilityOptions, args.Get(3))
	if groupVisibility == "" {
		groupVisibility = "private"
	}
	prompt.PrintNewLine()

	if groupName == "" || groupPath == "" {
		prompt.PrintError("Missing parameters, group name and group path are required")
		return "", errors.New("missing parameters")
	}

	data := map[string]string{
		"name":        groupName,
		"path":        groupPath,
		"description": groupDescription,
		"visibility":  groupVisibility,
	}

	createResp, err := gl.execPost("/groups", data, &glGroup{})

	if err != nil {
		prompt.PrintError("Cannot create group. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Group created")

	return strconv.Itoa(createResp.(*glGroup).Id), nil
}

func (gl *gitLab) createRepository(args cli.Args) (string, error) {

	groups, err := gl.getGroups()

	groups = funk.Filter(groups, func(g gitGroup) bool { return funk.ContainsString(gl.config.Git.GroupIds, g.Id) }).([]gitGroup)

	idx := 0
	groupId := ""
	if len(groups) == 0 {
		prompt.PrintError("No groups available")
		return "", errors.New("no groups available")
	} else if len(groups) == 1 {
		groupId = groups[0].Id
	} else {
		defaultGroupId := ""
		groupOptions := funk.Map(groups, func(group gitGroup) prompt.Option {
			if args.Get(0) == group.Name {
				defaultGroupId = group.Id
			}
			return prompt.Option{Id: group.Id, Name: group.Name}
		}).([]prompt.Option)

		groupId = prompt.Choice("Select your group :", groupOptions, defaultGroupId)
		idx = idx + 1
	}

	projectName := prompt.Input("\nEnter your project name :", args.Get(idx))
	projectDescription := prompt.Input("\nEnter your project description :", args.Get(idx+1))
	projectVisibility := prompt.Choice("\nSelect your project visibility ( default: private ) :", glVisibilityOptions, args.Get(idx+2))
	if projectVisibility == "" {
		projectVisibility = "private"
	}

	prompt.PrintNewLine()

	if groupId == "" || projectName == "" {
		prompt.PrintError("Missing parameters, project name and group are required")
		return "", errors.New("missing parameters")
	}

	data := map[string]string{
		"name":         projectName,
		"namespace_id": groupId,
		"description":  projectDescription,
		"visibility":   projectVisibility,
	}
	createResp, err := gl.execPost("projects", data, &glProject{})

	if err != nil {
		prompt.PrintError("Cannot create repository. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Repository created")

	return strconv.Itoa(createResp.(*glProject).Id), nil
}

func (gl *gitLab) getGroups() ([]gitGroup, error) {

	resp, err := gl.execGet("groups?per_page=100", &[]glGroup{})
	if err != nil {
		return nil, err
	}

	return funk.Map(*resp.(*[]glGroup), gl.toGitGroup).([]gitGroup), nil
}

func (gl *gitLab) getRepositories() ([]gitRepository, error) {

	var repos []gitRepository

	funk.ForEach(gl.config.Git.GroupIds, func(groupId string) {

		resp, err := gl.execGet("groups/"+groupId+"/projects?order_by=name&sort=asc&per_page=100", &[]glProject{})
		if err == nil {
			filteredRepos := *resp.(*[]glProject)
			if !gl.config.Git.IncludeArchivedProjects {
				filteredRepos = funk.Filter(filteredRepos, func(repo glProject) bool { return !repo.Archived }).([]glProject)
			}
			repos = append(repos, funk.Map(filteredRepos, gl.toGitRepo(groupId)).([]gitRepository)...)
		}
	})

	return repos, nil
}

func (gl *gitLab) createReviewRequest(group string, folder string, from string, into string, title string, message string) (reviewRequest, error) {

	// TODO ( Implement )
	return reviewRequest{}, nil
}

func (gl *gitLab) execGet(url string, resultType interface{}) (interface{}, error) {

	resp, err := resty.R().
		SetHeader("PRIVATE-TOKEN", gl.config.Git.PrivateToken).
		SetResult(resultType).
		Get(gl.apiUrl + "/" + url)

	if err != nil {
		prompt.PrintError(resp.String())
	}

	return resp.Result(), err
}

func (gl *gitLab) execPost(url string, data map[string]string, resultType interface{}) (interface{}, error) {

	resp, err := resty.R().
		SetHeader("PRIVATE-TOKEN", gl.config.Git.PrivateToken).
		SetResult(resultType).
		SetFormData(data).
		Post(gl.apiUrl + "/" + url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return nil, errors.New(fmt.Sprintf("Cannot execute post request. Status code : %d. Message : %s", resp.StatusCode(), resp.String()))
	}

	return resp.Result(), nil
}

func (gl *gitLab) toGitGroup(group glGroup) gitGroup {

	return gitGroup{
		Id:   strconv.Itoa(group.Id),
		Name: group.Name,
	}
}

func (gl *gitLab) toGitRepo(groupId string) interface{} {

	return func(project glProject) gitRepository {

		return gitRepository{
			Id:                strconv.Itoa(project.Id),
			Name:              project.Name,
			Description:       project.Description,
			NameWithNamespace: project.NameWithNamespace,
			Path:              project.Path,
			PathWithNamespace: project.PathWithNamespace,
			SshUrl:            project.SshUrl,
			HttpUrl:           project.HttpUrl,
			DefaultBranch:     project.DefaultBranch,
			Archived:          project.Archived,
			GroupId:           groupId,
		}
	}
}
