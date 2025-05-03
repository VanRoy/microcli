package git

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/thoas/go-funk"
	"github.com/urfave/cli/v3"
	"github.com/vanroy/microcli/impl/config"
	"github.com/vanroy/microcli/impl/prompt"
)

type gitHub struct {
	config config.Config
	labels Labels
	apiUrl string
}

type ghOrg struct {
	Id          int    `json:"id"`
	Login       string `json:"login"`
	Description string `json:"description"`
	AvatarUrl   string `json:"avatar_url"`
	WebUrl      string `json:"url"`
	ReposUrl    string `json:"repos_url"`
}

type ghRepo struct {
	Id                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	NameWithNamespace string `json:"full_name"`
	WebUrl            string `json:"url"`
	HttpUrl           string `json:"clone_url"`
	SshUrl            string `json:"ssh_url"`
	DefaultBranch     string `json:"default_branch"`
	Archived          bool   `json:"archived"`
}

var ghVisibilityOptions = []prompt.Option{
	{Id: "public", Name: "Public"},
	{Id: "private", Name: "Private"},
}

type ghCreateOrg struct {
	Login       string `json:"login"`
	Admin       string `json:"admin"`
	ProfileName string `json:"profile_name"`
}

type ghUpdateOrg struct {
	Description string `json:"description"`
}

type ghCreateRepository struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

type ghCreatePullRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
	Draft bool   `json:"draft"`
}

type ghPullRequest struct {
	Id        int    `json:"id"`
	HtmlUrl   string `json:"html_url"`
	State     string `json:"state"`
	Mergeable string `json:"mergeable"`
}

func newGitHub(config config.Config) gitRemote {

	baseUrl := config.Git.BaseUrl

	if !strings.Contains(baseUrl, "api.github.com") {
		if !strings.HasSuffix(baseUrl, "/") {
			baseUrl += "/"
		}
		baseUrl += "api/v3"
	}

	baseUrl = strings.TrimSuffix(baseUrl, "/")

	return &gitHub{
		config: config,
		labels: Labels{
			GroupLabel:            "organization",
			GroupsLabel:           "organizations",
			RepositoryLabel:       "repository",
			RepositoriesLabel:     "repositories",
			CreateGroupUsage:      "name path [description] [visibility]",
			CreateRepositoryUsage: "[group] name [description] [visibility]",
			CodeReviewRequest:     "pull request",
		},
		apiUrl: baseUrl,
	}
}

func (gh *gitHub) getLabels() Labels {
	return gh.labels
}

func (gh *gitHub) createGroup(args cli.Args) (string, error) {

	orgaLogin := prompt.Input("Enter your organization login :", args.Get(0))
	orgaName := prompt.Input("Enter your organization name :", args.Get(0))
	orgaAdminLogin := prompt.Input("\nEnter the login of the user who will manage this organization :", args.Get(1))
	orgaDescription := prompt.Input("\nEnter your group description :", args.Get(2))
	prompt.PrintNewLine()

	if orgaLogin == "" || orgaAdminLogin == "" {
		prompt.PrintErrorf("Missing parameters, group name and group admin login are required")
		return "", errors.New("missing parameters")
	}

	data := ghCreateOrg{
		Login:       orgaLogin,
		Admin:       orgaAdminLogin,
		ProfileName: orgaName,
	}

	createResp, err := gh.execPost("admin/organizations", data, &ghOrg{})
	if err != nil {
		prompt.PrintErrorf("Cannot create organization. ( %s )", err.Error())
		return "", err
	}

	orgId := strconv.Itoa(createResp.(*ghOrg).Id)

	if orgaDescription != "" {

		descData := ghUpdateOrg{
			Description: orgaDescription,
		}
		_, err := gh.execPatch("orgs/"+orgId, descData, &ghOrg{})
		if err != nil {
			prompt.PrintErrorf("Cannot create organization. ( %s )", err.Error())
			return "", err
		}

	}

	prompt.PrintInfo("Group created")

	return orgId, nil
}

func (gh *gitHub) createRepository(args cli.Args) (string, error) {

	groups, _ := gh.getGroups()

	groups = funk.Filter(groups, func(g gitGroup) bool { return funk.ContainsString(gh.config.Git.GroupIds, g.Id) }).([]gitGroup)

	idx := 0
	groupId := ""
	if len(groups) == 0 {
		prompt.PrintErrorf("No organization available")
		return "", errors.New("no organization available")
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

	projectName := prompt.Input("\nEnter your repository name :", args.Get(idx))
	projectDescription := prompt.Input("\nEnter your repository description :", args.Get(idx+1))
	projectVisibility := prompt.Choice("\nSelect your repository visibility ( default: private ) :", ghVisibilityOptions, args.Get(idx+2))

	projectPrivate := projectVisibility != "public"

	prompt.PrintNewLine()

	if groupId == "" || projectName == "" {
		prompt.PrintErrorf("Missing parameters, project name and group are required")
		return "", errors.New("missing parameters")
	}

	data := ghCreateRepository{
		Name:        projectName,
		Description: projectDescription,
		Private:     projectPrivate,
	}
	createResp, err := gh.execPost(gh.getGroupBasePath(groupId)+"/repos", data, &ghRepo{})

	if err != nil {
		prompt.PrintErrorf("Cannot create repository. ( %s )", err.Error())
		return "", err
	}

	prompt.PrintInfo("Repository created")

	return strconv.Itoa(createResp.(*ghRepo).Id), nil
}

func (gh *gitHub) getGroups() ([]gitGroup, error) {

	resp, _, err := gh.execGet("user/orgs", &[]ghOrg{})
	if err != nil {
		return nil, err
	}

	groups := funk.Map(*resp.(*[]ghOrg), gh.toGitGroup).([]gitGroup)

	// Add default personal group to allow user to read personal repositories
	groups = append(groups, PERSONAL_GROUP)

	return groups, nil
}

func (gh *gitHub) getRepositories() ([]gitRepository, error) {

	var repos []gitRepository

	funk.ForEach(gh.config.Git.GroupIds, func(groupId string) {

		var curPage = 1

		for {
			resp, nextPage, err := gh.execGet(gh.getGroupBasePath(groupId)+"/repos?type=sources&page="+strconv.Itoa(curPage), &[]ghRepo{})
			curPage = nextPage
			if err == nil {
				filteredRepos := *resp.(*[]ghRepo)
				if !gh.config.Git.IncludeArchivedProjects {
					filteredRepos = funk.Filter(filteredRepos, func(repo ghRepo) bool { return !repo.Archived }).([]ghRepo)
				}
				repos = append(repos, funk.Map(filteredRepos, gh.toGitRepo(groupId)).([]gitRepository)...)
				if nextPage == 0 {
					break
				}
			} else {
				break
			}
		}
	})

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].NameWithNamespace < repos[j].NameWithNamespace
	})

	return repos, nil
}

func (gh *gitHub) createReviewRequest(groupId string, repoId string, from string, into string, title string, message string, draft bool) (reviewRequest, error) {

	data := ghCreatePullRequest{
		Title: title,
		Head:  from,
		Base:  into,
		Body:  message,
		Draft: draft,
	}

	ghReview, err := gh.execPost("repos/"+groupId+"/"+repoId+"/pulls", data, &ghPullRequest{})
	if err != nil {
		return reviewRequest{}, err
	}

	return gh.toReviewRequest(ghReview.(*ghPullRequest)), err
}

func (gh *gitHub) execGet(url string, resultType interface{}) (interface{}, int, error) {

	resp, err := resty.New().R().
		SetHeader("Authorization", "token "+gh.config.Git.PrivateToken).
		SetResult(resultType).
		Get(gh.apiUrl + "/" + url)

	if err != nil {
		prompt.PrintError(resp.String())
		return nil, 0, err
	}

	nextPage := gh.parseNextPageValue(resp)
	return resp.Result(), nextPage, err
}

func (gh *gitHub) execPost(url string, data interface{}, resultType interface{}) (interface{}, error) {

	resp, err := resty.New().R().
		SetHeader("Authorization", "token "+gh.config.Git.PrivateToken).
		SetResult(resultType).
		SetBody(data).
		Post(gh.apiUrl + "/" + url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return nil, fmt.Errorf("cannot execute post request. Status code : %d. Message : %s", resp.StatusCode(), resp.String())
	}

	return resp.Result(), nil
}

func (gh *gitHub) execPatch(url string, data interface{}, resultType interface{}) (interface{}, error) {

	resp, err := resty.New().R().
		SetHeader("Authorization", "token "+gh.config.Git.PrivateToken).
		SetResult(resultType).
		SetBody(data).
		Patch(gh.apiUrl + "/" + url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 300 {
		return nil, fmt.Errorf("cannot execute post request. Status code : %d. Message : %s", resp.StatusCode(), resp.String())
	}

	return resp.Result(), nil
}

func (gh *gitHub) toGitGroup(org ghOrg) gitGroup {

	return gitGroup{
		Id:   org.Login,
		Name: org.Login,
	}
}

func (gh *gitHub) toGitRepo(groupId string) interface{} {

	return func(repository ghRepo) gitRepository {

		return gitRepository{
			Id:                strconv.Itoa(repository.Id),
			Name:              repository.Name,
			Description:       repository.Description,
			NameWithNamespace: repository.NameWithNamespace,
			Path:              gh.normalize(repository.Name),
			PathWithNamespace: gh.normalize(repository.NameWithNamespace),
			SshUrl:            repository.SshUrl,
			HttpUrl:           repository.HttpUrl,
			DefaultBranch:     repository.DefaultBranch,
			Archived:          repository.Archived,
			GroupId:           groupId,
		}
	}
}

func (gh *gitHub) toReviewRequest(request *ghPullRequest) reviewRequest {

	return reviewRequest{
		Id:        strconv.Itoa(request.Id),
		Url:       request.HtmlUrl,
		State:     request.State,
		Mergeable: request.Mergeable,
	}
}

func (gh *gitHub) getGroupBasePath(groupId string) string {
	if groupId == PERSONAL_GROUP.Id {
		return "user"
	} else {
		return "orgs/" + groupId
	}
}

func (gh *gitHub) parseNextPageValue(r *resty.Response) int {
	link := r.Header().Get("Link")
	if len(link) == 0 {
		return 0
	}

	for _, link := range strings.Split(link, ",") {
		segments := strings.Split(strings.TrimSpace(link), ";")

		// link must at least have href and rel
		if len(segments) < 2 {
			continue
		}

		// ensure href is properly formatted
		if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
			continue
		}

		// try to pull out page parameter
		parsedUrl, err := url.Parse(segments[0][1 : len(segments[0])-1])
		if err != nil {
			continue
		}
		page := parsedUrl.Query().Get("page")
		if page == "" {
			continue
		}

		for _, segment := range segments[1:] {
			switch strings.TrimSpace(segment) {
			case `rel="next"`:
				p, _ := strconv.Atoi(page)
				return p
			}
		}
	}

	return 0
}

func (gh *gitHub) normalize(name string) string {
	if !gh.config.Git.NormalizeName {
		return name
	}
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}
