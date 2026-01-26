package git

import "github.com/urfave/cli/v3"

type Labels struct {
	GroupLabel            string
	GroupsLabel           string
	RepositoryLabel       string
	RepositoriesLabel     string
	CreateGroupUsage      string
	CreateRepositoryUsage string
	CodeReviewRequest     string
}

type gitRepository struct {
	Id                string
	Name              string
	Path              string
	NameWithNamespace string
	PathWithNamespace string
	Description       string
	SshUrl            string
	HttpUrl           string
	GroupId           string
	DefaultBranch     string
	Archived          bool
}

type gitGroup struct {
	Id   string
	Name string
}

var PERSONAL_GROUP = gitGroup{
	Id:   "PERSONAL",
	Name: "Personal",
}

type reviewRequest struct {
	Id        string
	State     string
	Title     string
	Url       string
	Mergeable string
}

type gitRemote interface {
	getLabels() Labels
	createGroup(args cli.Args) (string, error)
	createRepository(args cli.Args) (string, error)
	getGroups() ([]gitGroup, error)
	getRepositories() ([]gitRepository, error)
	createReviewRequest(repository *gitRepository, from string, into string, title string, message string, draft bool) (reviewRequest, error)
}
